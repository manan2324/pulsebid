package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/platform/clock"
)

// Snapshot represents an immutable snapshot of the catalog used by the gateway.
type Snapshot struct {
	Version       int64
	Campaigns     []*CompiledCampaign
	CampaignsByID map[string]*CompiledCampaign
	Bidders       map[string]*catalogv1.Bidder
	GeneratedAt   time.Time
	LoadedAt      time.Time
}

// SnapshotCache holds an atomic pointer to the active Snapshot.
type SnapshotCache struct {
	current atomic.Pointer[Snapshot]
	metrics *Metrics
	logger  *slog.Logger
	clk     clock.Clock
}

// NewSnapshotCache creates a new uninitialized snapshot cache.
func NewSnapshotCache(m *Metrics, logger *slog.Logger, clk clock.Clock) *SnapshotCache {
	if clk == nil {
		clk = clock.Real{}
	}
	return &SnapshotCache{
		metrics: m,
		logger:  logger,
		clk:     clk,
	}
}

// Get returns the current active snapshot, or nil if none is loaded yet.
func (c *SnapshotCache) Get() *Snapshot {
	return c.current.Load()
}

// IsReady returns true once at least one valid snapshot has been loaded.
func (c *SnapshotCache) IsReady() bool {
	return c.current.Load() != nil
}

// CheckReady satisfies the health.Check signature for /readyz.
func (c *SnapshotCache) CheckReady(context.Context) error {
	if !c.IsReady() {
		return errors.New("initial catalog snapshot not loaded")
	}
	return nil
}

// Set compiles and atomically replaces the active snapshot.
func (c *SnapshotCache) Set(resp *catalogv1.GetSnapshotResponse) *Snapshot {
	if resp == nil {
		return nil
	}

	campaigns := make([]*CompiledCampaign, 0, len(resp.GetCampaigns()))
	campaignsByID := make(map[string]*CompiledCampaign, len(resp.GetCampaigns()))
	for _, raw := range resp.GetCampaigns() {
		cc := CompileCampaign(raw)
		campaigns = append(campaigns, cc)
		campaignsByID[cc.ID] = cc
	}

	bidders := make(map[string]*catalogv1.Bidder, len(resp.GetBidders()))
	for _, b := range resp.GetBidders() {
		bidders[b.GetId()] = b
	}

	var genTime time.Time
	if resp.GetGeneratedAt() != nil {
		genTime = resp.GetGeneratedAt().AsTime().UTC()
	} else {
		genTime = c.clk.Now().UTC()
	}

	snap := &Snapshot{
		Version:       resp.GetVersion(),
		Campaigns:     campaigns,
		CampaignsByID: campaignsByID,
		Bidders:       bidders,
		GeneratedAt:   genTime,
		LoadedAt:      c.clk.Now().UTC(),
	}

	c.current.Store(snap)
	if c.metrics != nil {
		c.metrics.SnapshotVersion.Set(float64(snap.Version))
		c.metrics.SnapshotCampaigns.Set(float64(len(snap.Campaigns)))
	}
	return snap
}

// Poll fetches the catalog snapshot from campaign-service over gRPC.
func (c *SnapshotCache) Poll(ctx context.Context, client catalogv1.CatalogServiceClient) error {
	var knownVersion int64
	if current := c.Get(); current != nil {
		knownVersion = current.Version
	}

	resp, err := client.GetSnapshot(ctx, &catalogv1.GetSnapshotRequest{
		KnownVersion: knownVersion,
	})
	if err != nil {
		c.logger.WarnContext(ctx, "failed to poll catalog snapshot; serving stale", "error", err, "known_version", knownVersion)
		return err
	}

	if resp.GetUnchanged() {
		if current := c.Get(); current != nil && c.metrics != nil {
			c.metrics.SnapshotAgeSeconds.Set(c.clk.Now().UTC().Sub(current.GeneratedAt).Seconds())
		}
		return nil
	}

	snap := c.Set(resp)
	if c.metrics != nil && snap != nil {
		c.metrics.SnapshotAgeSeconds.Set(c.clk.Now().UTC().Sub(snap.GeneratedAt).Seconds())
	}
	c.logger.InfoContext(ctx, "catalog snapshot refreshed",
		"version", resp.GetVersion(),
		"campaigns", len(resp.GetCampaigns()),
		"bidders", len(resp.GetBidders()))
	return nil
}

// RunRefresher continuously polls the catalog snapshot in the background.
func (c *SnapshotCache) RunRefresher(ctx context.Context, client catalogv1.CatalogServiceClient, interval time.Duration) error {
	// Initial poll immediately
	_ = c.Poll(ctx, client)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = c.Poll(ctx, client)
		}
	}
}
