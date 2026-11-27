package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	db "pulsebid/gen/campaigndb"
	"pulsebid/internal/budget"
	"pulsebid/internal/platform/ids"
)

// Service owns input validation, state transitions, and transactional mutations.
type Service struct {
	repo                Repository
	reserver            budget.Reserver
	enableTestEndpoints bool
}

// NewService injects the persistence boundary.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// WithBudget configures the budget reserver and test endpoint toggle.
func (s *Service) WithBudget(reserver budget.Reserver, enableTestEndpoints bool) *Service {
	s.reserver = reserver
	s.enableTestEndpoints = enableTestEndpoints
	return s
}

// GetBudget reads the current daily spend and remaining budget for a campaign.
func (s *Service) GetBudget(ctx context.Context, id string, now time.Time) (map[string]any, error) {
	c, err := s.Campaign(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.reserver == nil {
		return nil, ErrUnavailable
	}

	spentMicros, err := s.reserver.Spent(ctx, c.ID, now)
	if err != nil {
		return nil, fmt.Errorf("read spend: %w", err)
	}

	dailyBudgetMicros := int64(c.DailyBudget)
	dailyBudget := float64(dailyBudgetMicros) / 1_000_000.0
	spent := float64(spentMicros) / 1_000_000.0
	remMicros := dailyBudgetMicros - spentMicros
	if remMicros < 0 {
		remMicros = 0
	}
	remaining := float64(remMicros) / 1_000_000.0

	return map[string]any{
		"campaign_id":         c.ID,
		"date":                now.Format("2006-01-02"),
		"daily_budget":        dailyBudget,
		"daily_budget_micros": dailyBudgetMicros,
		"spent":               spent,
		"spent_micros":        spentMicros,
		"remaining":           remaining,
		"remaining_micros":    remMicros,
	}, nil
}

// ResetBudget resets a campaign's spend in test mode.
func (s *Service) ResetBudget(ctx context.Context, id string, now time.Time) error {
	if !s.enableTestEndpoints {
		return ErrNotFound
	}
	c, err := s.Campaign(ctx, id)
	if err != nil {
		return err
	}
	if s.reserver == nil {
		return ErrUnavailable
	}
	return s.reserver.Reset(ctx, c.ID, now)
}

// Campaign fetches a campaign by UUID.
func (s *Service) Campaign(ctx context.Context, id string) (Campaign, error) {
	if !validUUID(id) {
		return Campaign{}, invalid("id", "must be a UUID")
	}
	var row db.Campaign
	err := s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		row, err = q.GetCampaign(ctx, id)
		return err
	})
	return campaignFromDB(row), err
}

// CreateCampaign always starts a campaign in draft state.
func (s *Service) CreateCampaign(ctx context.Context, body map[string]json.RawMessage) (Campaign, error) {
	c, err := merge(Campaign{Status: "draft", PacingMode: "even"}, body, "advertiser_id bidder_id name daily_budget max_bid_cpm pacing_mode start_at end_at", "start_at end_at")
	if err != nil {
		return Campaign{}, err
	}
	if err = validateCampaign(&c); err != nil {
		return Campaign{}, err
	}
	c.ID, err = ids.New()
	if err != nil {
		return Campaign{}, err
	}
	var row db.Campaign
	err = s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		row, err = q.CreateCampaign(ctx, db.CreateCampaignParams(campaignParams(c)))
		return err
	})
	return campaignFromDB(row), err
}

// PatchCampaign locks and merges the current record, preventing lost updates.
func (s *Service) PatchCampaign(ctx context.Context, id string, body map[string]json.RawMessage) (Campaign, error) {
	if !validUUID(id) {
		return Campaign{}, invalid("id", "must be a UUID")
	}
	var result db.Campaign
	err := s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		row, err := q.GetCampaign(ctx, id)
		if err != nil {
			return err
		}
		old := campaignFromDB(row)
		next, err := merge(old, body, "advertiser_id bidder_id name status daily_budget max_bid_cpm pacing_mode start_at end_at", "start_at end_at")
		if err != nil {
			return err
		}
		if err = validateCampaign(&next); err != nil {
			return err
		}
		if err = transition(ctx, q, old, next); err != nil {
			return err
		}
		result, err = q.UpdateCampaign(ctx, campaignParams(next))
		return err
	})
	return campaignFromDB(result), err
}

// SetCampaignStatus uses the same state checks as PATCH.
func (s *Service) SetCampaignStatus(ctx context.Context, id, status string) (Campaign, error) {
	encoded, _ := json.Marshal(status)
	return s.PatchCampaign(ctx, id, map[string]json.RawMessage{"status": encoded})
}

func transition(ctx context.Context, q *db.Queries, old, next Campaign) error {
	if old.Status == "archived" && next.Status != "archived" || next.Status == "draft" && old.Status != "draft" || old.Status == "draft" && next.Status == "paused" {
		return fmt.Errorf("%w: invalid campaign state transition", ErrConflict)
	}
	if next.Status == "active" {
		present, err := q.HasActiveCreative(ctx, next.ID)
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("%w: activation requires an active creative", ErrConflict)
		}
	}
	return nil
}

func campaignParams(c Campaign) db.UpdateCampaignParams {
	return db.UpdateCampaignParams{ID: c.ID, AdvertiserID: c.AdvertiserID, BidderID: c.BidderID, Name: c.Name, Status: c.Status, DailyBudgetMicros: int64(c.DailyBudget), MaxBidCpmMicros: int64(c.MaxBidCPM), PacingMode: c.PacingMode, StartAt: c.StartAt, EndAt: c.EndAt}
}

// Campaigns lists matching campaigns in descending (created_at,id) order.
func (s *Service) Campaigns(ctx context.Context, query url.Values) (Page[Campaign], error) {
	advertiser, status := query.Get("advertiser_id"), query.Get("status")
	if advertiser != "" && !validUUID(advertiser) {
		return Page[Campaign]{}, invalid("advertiser_id", "must be a UUID")
	}
	if status != "" && !validStatus(status) {
		return Page[Campaign]{}, invalid("status", "unknown campaign status")
	}
	p, err := parsePage(query, "campaigns|"+advertiser+"|"+status)
	if err != nil {
		return Page[Campaign]{}, err
	}
	var rows []db.Campaign
	err = s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		rows, err = q.ListCampaigns(ctx, db.ListCampaignsParams{HasCursor: p.HasCursor, CursorTime: p.Time, CursorID: p.ID, PageLimit: p.Limit + 1, AdvertiserFilter: advertiser, StatusFilter: status})
		return err
	})
	if err != nil {
		return Page[Campaign]{}, err
	}
	result := make([]Campaign, 0, len(rows))
	for _, row := range rows {
		result = append(result, campaignFromDB(row))
	}
	return paginate(result, p, func(c Campaign) (time.Time, string) { return c.CreatedAt, c.ID }), nil
}

// Targeting returns unrestricted defaults when a campaign has no rule yet.
func (s *Service) Targeting(ctx context.Context, id string) (Targeting, error) {
	if !validUUID(id) {
		return Targeting{}, invalid("id", "must be a UUID")
	}
	var result Targeting
	err := s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		c, err := q.GetCampaign(ctx, id)
		if err != nil {
			return err
		}
		result, err = q.GetTargeting(ctx, id)
		if databaseError(err) == ErrNotFound {
			result = Targeting{CampaignID: id, Countries: []string{}, DeviceTypes: []string{}, PlacementIds: []string{}, HoursOfDay: []int16{}, Timezone: "UTC", UpdatedAt: c.CreatedAt}
			return nil
		}
		return err
	})
	return result, err
}

// PutTargeting fully replaces restrictions and creates a missing rule.
func (s *Service) PutTargeting(ctx context.Context, id string, body map[string]json.RawMessage) (Targeting, error) {
	if !validUUID(id) {
		return Targeting{}, invalid("id", "must be a UUID")
	}
	rule, err := merge(Targeting{CampaignID: id, Timezone: "UTC"}, body, "countries device_types placement_ids hours_of_day timezone", "")
	if err != nil {
		return Targeting{}, err
	}
	if err = normalizeTargeting(&rule); err != nil {
		return Targeting{}, err
	}
	var result Targeting
	err = s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		if _, err := q.GetCampaign(ctx, id); err != nil {
			return err
		}
		var err error
		result, err = q.PutTargeting(ctx, db.PutTargetingParams{CampaignID: id, Countries: rule.Countries, DeviceTypes: rule.DeviceTypes, PlacementIds: rule.PlacementIds, HoursOfDay: rule.HoursOfDay, Timezone: rule.Timezone})
		return err
	})
	return result, err
}
