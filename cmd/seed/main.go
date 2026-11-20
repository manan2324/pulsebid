// Command seed idempotently registers mock bidders, an advertiser, and active campaigns for testing.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pulsebid/internal/platform/app"
	"pulsebid/internal/platform/config"
	"pulsebid/internal/platform/logging"
)

func main() {
	// If ADMIN_ADDR is passed without RUN_SEED=1 (e.g. during make test-platform smoke test),
	// run the platform admin shell to preserve smoke-platform.sh compatibility.
	if os.Getenv("ADMIN_ADDR") != "" && os.Getenv("RUN_SEED") != "1" {
		if err := runSmokeShell(); err != nil {
			logging.New(os.Stdout, "seed", "dev", slog.LevelInfo).Error("service failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := runSeed(); err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}
}

func runSmokeShell() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadCommon("seed", ":8095")
	if err != nil {
		return err
	}
	return app.Run(ctx, cfg, "dev", os.Stdout)
}

type client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newClient(baseURL, apiKey string) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *client) do(ctx context.Context, method, path string, in any, out any) (int, error) {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if out != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response from %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

func runSeed() error {
	campaignURL := os.Getenv("CAMPAIGN_URL")
	if campaignURL == "" {
		campaignURL = "http://localhost:8081"
	}
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("API_KEYS")
	}
	if apiKey == "" {
		apiKey = "dev-key"
	}
	bidderHost := os.Getenv("BIDDER_HOST")
	if bidderHost == "" {
		bidderHost = "http://localhost"
	}
	bidderHost = strings.TrimRight(bidderHost, "/")

	bidderCount := 8
	if val := os.Getenv("BIDDER_COUNT"); val != "" {
		if c, err := strconv.Atoi(val); err == nil && c > 0 {
			bidderCount = c
		}
	}
	basePort := 9101
	if val := os.Getenv("BIDDER_BASE_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			basePort = p
		}
	}
	seedCampaigns := 40
	if val := os.Getenv("SEED_CAMPAIGNS"); val != "" {
		if s, err := strconv.Atoi(val); err == nil && s > 0 {
			seedCampaigns = s
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cli := newClient(campaignURL, apiKey)

	fmt.Printf("Starting seed: campaign_url=%s, bidders=%d, campaigns=%d, bidder_host=%s\n",
		campaignURL, bidderCount, seedCampaigns, bidderHost)

	// 1. Idempotently register bidders
	for i := 0; i < bidderCount; i++ {
		id := fmt.Sprintf("bidder-%d", i+1)
		port := basePort + i
		endpoint := fmt.Sprintf("%s:%d/bid", bidderHost, port)
		timeoutMs := 100

		bidderReq := map[string]any{
			"id":           id,
			"name":         fmt.Sprintf("Mock Bidder %d", i+1),
			"endpoint_url": endpoint,
			"timeout_ms":   timeoutMs,
			"enabled":      true,
		}

		status, err := cli.do(ctx, http.MethodPost, "/v1/bidders", bidderReq, nil)
		if err != nil {
			return fmt.Errorf("register bidder %s: %w", id, err)
		}
		if status == http.StatusConflict {
			// Update endpoint in case host or port changed
			patchReq := map[string]any{
				"endpoint_url": endpoint,
				"enabled":      true,
			}
			_, _ = cli.do(ctx, http.MethodPatch, "/v1/bidders/"+id, patchReq, nil)
		} else if status >= 400 {
			return fmt.Errorf("register bidder %s returned HTTP %d", id, status)
		}
	}
	fmt.Printf("Registered %d bidders.\n", bidderCount)

	// 2. Idempotently create/find advertiser
	advName := "Seed Advertiser"
	var advertiserID string

	advReq := map[string]any{
		"name":          advName,
		"contact_email": "seed@pulsebid.test",
	}
	var advResp struct {
		ID string `json:"id"`
	}
	status, err := cli.do(ctx, http.MethodPost, "/v1/advertisers", advReq, &advResp)
	if err != nil {
		return fmt.Errorf("create advertiser: %w", err)
	}
	if status == http.StatusCreated {
		advertiserID = advResp.ID
	} else if status == http.StatusConflict {
		// Fetch existing advertiser
		var page struct {
			Items []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"items"`
		}
		_, err := cli.do(ctx, http.MethodGet, "/v1/advertisers?limit=200", nil, &page)
		if err != nil {
			return fmt.Errorf("list advertisers: %w", err)
		}
		for _, item := range page.Items {
			if strings.EqualFold(item.Name, advName) {
				advertiserID = item.ID
				break
			}
		}
	} else {
		return fmt.Errorf("create advertiser returned HTTP %d", status)
	}

	if advertiserID == "" {
		return errors.New("failed to find or create seed advertiser")
	}
	fmt.Printf("Seed advertiser ready: %s\n", advertiserID)

	// 3. Check existing campaigns for this advertiser
	var campPage struct {
		Items []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"items"`
	}
	_, err = cli.do(ctx, http.MethodGet, "/v1/campaigns?advertiser_id="+advertiserID+"&limit=200", nil, &campPage)
	if err != nil {
		return fmt.Errorf("list existing campaigns: %w", err)
	}

	existingCount := len(campPage.Items)
	// Ensure existing campaigns are active
	for _, c := range campPage.Items {
		if c.Status == "draft" || c.Status == "paused" {
			_, _ = cli.do(ctx, http.MethodPost, fmt.Sprintf("/v1/campaigns/%s/activate", c.ID), nil, nil)
		}
	}

	countriesCycle := [][]string{
		{"IN"},
		{"US"},
		{"GB"},
		{"DE"},
		{"IN", "US"},
		{},
	}
	devicesCycle := [][]string{
		{"mobile"},
		{"desktop"},
		{"tablet"},
		{},
	}

	createdCount := 0
	for i := existingCount; i < seedCampaigns; i++ {
		bidderID := fmt.Sprintf("bidder-%d", (i%bidderCount)+1)
		campName := fmt.Sprintf("Seed Campaign %d", i+1)
		maxBid := 2.00 + float64(i%4)*0.50
		pacing := "even"
		if i%2 != 0 {
			pacing = "asap"
		}

		cReq := map[string]any{
			"advertiser_id": advertiserID,
			"bidder_id":     bidderID,
			"name":          campName,
			"daily_budget":  50000.00,
			"max_bid_cpm":   maxBid,
			"pacing_mode":   pacing,
		}

		var createdCamp struct {
			ID string `json:"id"`
		}
		status, err := cli.do(ctx, http.MethodPost, "/v1/campaigns", cReq, &createdCamp)
		if err != nil {
			return fmt.Errorf("create campaign %s: %w", campName, err)
		}
		if status != http.StatusCreated {
			return fmt.Errorf("create campaign %s returned HTTP %d", campName, status)
		}

		// Set targeting
		targetReq := map[string]any{
			"countries":     countriesCycle[i%len(countriesCycle)],
			"device_types":  devicesCycle[i%len(devicesCycle)],
			"placement_ids": []string{fmt.Sprintf("slot-%d", i%10)},
			"timezone":      "UTC",
		}
		status, err = cli.do(ctx, http.MethodPut, fmt.Sprintf("/v1/campaigns/%s/targeting", createdCamp.ID), targetReq, nil)
		if err != nil || status >= 400 {
			return fmt.Errorf("set targeting for %s: status=%d err=%v", createdCamp.ID, status, err)
		}

		// Add creative
		crReq := map[string]any{
			"name":        fmt.Sprintf("Banner 300x250 for %s", campName),
			"format":      "banner",
			"width":       300,
			"height":      250,
			"markup":      `<a href="{click_url}"><img src="https://cdn.test/seed.png"/></a><img src="{impression_url}" width="1" height="1"/>`,
			"landing_url": "https://seed.test/landing",
		}
		status, err = cli.do(ctx, http.MethodPost, fmt.Sprintf("/v1/campaigns/%s/creatives", createdCamp.ID), crReq, nil)
		if err != nil || status != http.StatusCreated {
			return fmt.Errorf("create creative for %s: status=%d err=%v", createdCamp.ID, status, err)
		}

		// Activate campaign
		status, err = cli.do(ctx, http.MethodPost, fmt.Sprintf("/v1/campaigns/%s/activate", createdCamp.ID), nil, nil)
		if err != nil || status != http.StatusOK {
			return fmt.Errorf("activate campaign %s: status=%d err=%v", createdCamp.ID, status, err)
		}
		createdCount++
	}

	fmt.Printf("Seeding complete: %d existing, %d newly created active campaigns (total %d).\n",
		existingCount, createdCount, existingCount+createdCount)
	return nil
}
