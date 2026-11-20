package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	db "pulsebid/gen/campaigndb"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
	"pulsebid/internal/platform/clock"
)

// SnapshotRepository provides a cheap version read and a coherent full catalog.
type SnapshotRepository interface {
	CatalogVersion(context.Context) (int64, error)
	CatalogSnapshot(context.Context) (*catalogv1.GetSnapshotResponse, error)
}

// SnapshotService caches immutable snapshots; only cache misses acquire the gate.
// The gate is context-aware and creates no background worker or abandoned waiter.
type SnapshotService struct {
	repo   SnapshotRepository
	clock  clock.Clock
	cached atomic.Pointer[catalogv1.GetSnapshotResponse]
	gate   chan struct{}
}

// NewSnapshotService injects the catalog source and generation timestamp clock.
func NewSnapshotService(repo SnapshotRepository, c clock.Clock) *SnapshotService {
	return &SnapshotService{repo: repo, clock: c, gate: make(chan struct{}, 1)}
}

// GetSnapshot returns an independent response, or a version-only unchanged result.
// Database errors never masquerade as a successful unchanged response.
func (s *SnapshotService) GetSnapshot(ctx context.Context, knownVersion int64) (*catalogv1.GetSnapshotResponse, error) {
	if knownVersion < 0 {
		return nil, invalid("known_version", "must be nonnegative")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	version, err := s.repo.CatalogVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version == knownVersion {
		return unchangedSnapshot(version), nil
	}
	if cached := s.cached.Load(); cached != nil && cached.Version == version {
		return snapshotResponse(cached, knownVersion), nil
	}
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.gate }()
	// Another caller may have rebuilt while we waited. Recheck the DB version
	// rather than publishing an older snapshot or rebuilding a current one.
	version, err = s.repo.CatalogVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version == knownVersion {
		return unchangedSnapshot(version), nil
	}
	if cached := s.cached.Load(); cached != nil && cached.Version == version {
		return snapshotResponse(cached, knownVersion), nil
	}
	next, err := s.repo.CatalogSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if next == nil || next.Version < version {
		return nil, fmt.Errorf("inconsistent catalog snapshot version")
	}
	next.GeneratedAt = timestamppb.New(s.clock.Now())
	if err = next.GeneratedAt.CheckValid(); err != nil {
		return nil, fmt.Errorf("snapshot generation time: %w", err)
	}
	s.cached.Store(next)
	return snapshotResponse(next, knownVersion), nil
}

func unchangedSnapshot(version int64) *catalogv1.GetSnapshotResponse {
	return &catalogv1.GetSnapshotResponse{Version: version, Unchanged: true}
}
func snapshotResponse(snapshot *catalogv1.GetSnapshotResponse, known int64) *catalogv1.GetSnapshotResponse {
	if snapshot.Version == known {
		return unchangedSnapshot(known)
	}
	return proto.Clone(snapshot).(*catalogv1.GetSnapshotResponse)
}

// CatalogVersion reads only the version row, under the repository deadline.
func (s *Store) CatalogVersion(ctx context.Context) (int64, error) {
	var version int64
	err := s.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		version, err = q.CatalogVersion(ctx)
		return err
	})
	return version, err
}

// CatalogSnapshot loads all related records in one SQL statement/MVCC snapshot.
func (s *Store) CatalogSnapshot(ctx context.Context) (*catalogv1.GetSnapshotResponse, error) {
	var row db.CatalogSnapshotRow
	err := s.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		row, err = q.CatalogSnapshot(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return decodeSnapshot(row)
}

// SQL uses JSON only as a typed aggregation format; no money passes through floats.
type snapshotCampaign struct {
	ID                string                `json:"id"`
	AdvertiserID      string                `json:"advertiser_id"`
	BidderID          string                `json:"bidder_id"`
	Name              string                `json:"name"`
	DailyBudgetMicros int64                 `json:"daily_budget_micros"`
	MaxBidCPMMicros   int64                 `json:"max_bid_cpm_micros"`
	PacingMode        string                `json:"pacing_mode"`
	StartAt           *time.Time            `json:"start_at"`
	EndAt             *time.Time            `json:"end_at"`
	Targeting         *catalogv1.Targeting  `json:"targeting"`
	Creatives         []*catalogv1.Creative `json:"creatives"`
}

func decodeSnapshot(row db.CatalogSnapshotRow) (*catalogv1.GetSnapshotResponse, error) {
	result := &catalogv1.GetSnapshotResponse{Version: row.Version, Campaigns: []*catalogv1.Campaign{}}
	if err := json.Unmarshal(row.Bidders, &result.Bidders); err != nil {
		return nil, fmt.Errorf("decode snapshot bidders: %w", err)
	}
	var campaigns []snapshotCampaign
	if err := json.Unmarshal(row.Campaigns, &campaigns); err != nil {
		return nil, fmt.Errorf("decode snapshot campaigns: %w", err)
	}
	for _, c := range campaigns {
		pacing := catalogv1.PacingMode_PACING_MODE_EVEN
		switch c.PacingMode {
		case "asap":
			pacing = catalogv1.PacingMode_PACING_MODE_ASAP
		case "even":
		default:
			return nil, fmt.Errorf("unknown stored pacing mode")
		}
		start, err := snapshotTimestamp(c.StartAt)
		if err != nil {
			return nil, err
		}
		end, err := snapshotTimestamp(c.EndAt)
		if err != nil {
			return nil, err
		}
		result.Campaigns = append(result.Campaigns, &catalogv1.Campaign{Id: c.ID, AdvertiserId: c.AdvertiserID, BidderId: c.BidderID, Name: c.Name, DailyBudgetMicros: c.DailyBudgetMicros, MaxBidCpmMicros: c.MaxBidCPMMicros, PacingMode: pacing, StartAt: start, EndAt: end, Targeting: c.Targeting, Creatives: c.Creatives})
	}
	return result, nil
}

func snapshotTimestamp(value *time.Time) (*timestamppb.Timestamp, error) {
	if value == nil {
		return nil, nil
	}
	stamp := timestamppb.New(*value)
	if err := stamp.CheckValid(); err != nil {
		return nil, fmt.Errorf("stored snapshot timestamp: %w", err)
	}
	return stamp, nil
}
