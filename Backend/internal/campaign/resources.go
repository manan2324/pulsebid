package campaign

import (
	"context"
	"encoding/json"
	"net/url"
	db "pulsebid/gen/campaigndb"
	"pulsebid/internal/platform/ids"
	"time"
)

// Advertiser fetches one advertiser.
func (s *Service) Advertiser(ctx context.Context, id string) (Advertiser, error) {
	if !validUUID(id) {
		return Advertiser{}, invalid("id", "invalid uuid")
	}
	var result Advertiser
	err := s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		result, err = q.GetAdvertiser(ctx, id)
		return err
	})
	return result, err
}

// CreateAdvertiser validates and persists a new record.
func (s *Service) CreateAdvertiser(ctx context.Context, body map[string]json.RawMessage) (Advertiser, error) {

	v, err := merge(Advertiser{Status: "active"}, body, "name contact_email", "contact_email")
	if err != nil {
		return Advertiser{}, err
	}
	if err = validateAdvertiser(v); err != nil {
		return Advertiser{}, err
	}
	v.ID, err = ids.New()
	if err != nil {
		return Advertiser{}, err
	}
	var result Advertiser
	err = s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {

		var err error
		result, err = q.CreateAdvertiser(ctx, db.CreateAdvertiserParams{ID: v.ID, Name: v.Name, ContactEmail: v.ContactEmail, Status: v.Status})
		return err
	})
	return result, err
}

// PatchAdvertiser merges permitted fields while holding the record lock.
func (s *Service) PatchAdvertiser(ctx context.Context, id string, body map[string]json.RawMessage) (Advertiser, error) {
	if !validUUID(id) {
		return Advertiser{}, invalid("id", "invalid uuid")
	}
	var result Advertiser
	err := s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		old, err := q.GetAdvertiser(ctx, id)
		if err != nil {
			return err
		}

		v, err := merge(old, body, "name contact_email status", "contact_email")
		if err != nil {
			return err
		}
		if err = validateAdvertiser(v); err != nil {
			return err
		}
		result, err = q.UpdateAdvertiser(ctx, db.UpdateAdvertiserParams{ID: v.ID, Name: v.Name, ContactEmail: v.ContactEmail, Status: v.Status})
		return err
	})
	return result, err
}

// Advertisers returns a bounded keyset page.
func (s *Service) Advertisers(ctx context.Context, query url.Values) (Page[Advertiser], error) {

	p, err := parsePage(query, "advertisers")
	if err != nil {
		return Page[Advertiser]{}, err
	}
	var rows []Advertiser
	err = s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {

		var err error
		rows, err = q.ListAdvertisers(ctx, db.ListAdvertisersParams{HasCursor: p.HasCursor, CursorTime: p.Time, CursorID: p.ID, PageLimit: p.Limit + 1})
		return err
	})
	if err != nil {
		return Page[Advertiser]{}, err
	}
	return paginate(rows, p, func(v Advertiser) (time.Time, string) { return v.CreatedAt, v.ID }), nil
}

// Bidder fetches one bidder.
func (s *Service) Bidder(ctx context.Context, id string) (Bidder, error) {
	if !validSlug(id) {
		return Bidder{}, invalid("id", "invalid slug")
	}
	var result Bidder
	err := s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		result, err = q.GetBidder(ctx, id)
		return err
	})
	return result, err
}

// CreateBidder validates and persists a new record.
func (s *Service) CreateBidder(ctx context.Context, body map[string]json.RawMessage) (Bidder, error) {

	v, err := merge(Bidder{Enabled: true}, body, "id name endpoint_url timeout_ms enabled", "timeout_ms")
	if err != nil {
		return Bidder{}, err
	}
	if err = validateBidder(v); err != nil {
		return Bidder{}, err
	}

	var result Bidder
	err = s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {

		var err error
		result, err = q.CreateBidder(ctx, db.CreateBidderParams{ID: v.ID, Name: v.Name, EndpointUrl: v.EndpointUrl, TimeoutMs: v.TimeoutMs, Enabled: v.Enabled})
		return err
	})
	return result, err
}

// PatchBidder merges permitted fields while holding the record lock.
func (s *Service) PatchBidder(ctx context.Context, id string, body map[string]json.RawMessage) (Bidder, error) {
	if !validSlug(id) {
		return Bidder{}, invalid("id", "invalid slug")
	}
	var result Bidder
	err := s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		old, err := q.GetBidder(ctx, id)
		if err != nil {
			return err
		}

		v, err := merge(old, body, "name endpoint_url timeout_ms enabled", "timeout_ms")
		if err != nil {
			return err
		}
		if err = validateBidder(v); err != nil {
			return err
		}
		result, err = q.UpdateBidder(ctx, db.UpdateBidderParams{ID: v.ID, Name: v.Name, EndpointUrl: v.EndpointUrl, TimeoutMs: v.TimeoutMs, Enabled: v.Enabled})
		return err
	})
	return result, err
}

// Bidders returns a bounded keyset page.
func (s *Service) Bidders(ctx context.Context, query url.Values) (Page[Bidder], error) {

	p, err := parsePage(query, "bidders")
	if err != nil {
		return Page[Bidder]{}, err
	}
	var rows []Bidder
	err = s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {

		var err error
		rows, err = q.ListBidders(ctx, db.ListBiddersParams{HasCursor: p.HasCursor, CursorTime: p.Time, CursorID: p.ID, PageLimit: p.Limit + 1})
		return err
	})
	if err != nil {
		return Page[Bidder]{}, err
	}
	return paginate(rows, p, func(v Bidder) (time.Time, string) { return v.CreatedAt, v.ID }), nil
}

// DeleteBidder removes a record; referenced bidders remain protected by foreign keys.
func (s *Service) DeleteBidder(ctx context.Context, id string) error {
	if !validSlug(id) {
		return invalid("id", "invalid slug")
	}
	return s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {

		count, err := q.DeleteBidder(ctx, id)
		if err != nil {
			return err
		}
		if count == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Creative fetches one creative.
func (s *Service) Creative(ctx context.Context, id string) (Creative, error) {
	if !validUUID(id) {
		return Creative{}, invalid("id", "invalid uuid")
	}
	var result Creative
	err := s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		var err error
		result, err = q.GetCreative(ctx, id)
		return err
	})
	return result, err
}

// CreateCreative validates and persists a new record.
func (s *Service) CreateCreative(ctx context.Context, parent string, body map[string]json.RawMessage) (Creative, error) {
	if !validUUID(parent) {
		return Creative{}, invalid("campaign_id", "must be a UUID")
	}
	v, err := merge(Creative{CampaignID: parent, Status: "active"}, body, "name format width height markup landing_url status", "width height")
	if err != nil {
		return Creative{}, err
	}
	if err = validateCreative(v); err != nil {
		return Creative{}, err
	}
	v.ID, err = ids.New()
	if err != nil {
		return Creative{}, err
	}
	var result Creative
	err = s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		if _, err := q.GetCampaign(ctx, parent); err != nil {
			return err
		}
		var err error
		result, err = q.CreateCreative(ctx, db.CreateCreativeParams{ID: v.ID, CampaignID: v.CampaignID, Name: v.Name, Format: v.Format, Width: v.Width, Height: v.Height, Markup: v.Markup, LandingUrl: v.LandingUrl, Status: v.Status})
		return err
	})
	return result, err
}

// PatchCreative merges permitted fields while holding the record lock.
func (s *Service) PatchCreative(ctx context.Context, id string, body map[string]json.RawMessage) (Creative, error) {
	if !validUUID(id) {
		return Creative{}, invalid("id", "invalid uuid")
	}
	var result Creative
	err := s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		old, err := q.GetCreative(ctx, id)
		if err != nil {
			return err
		}
		if _, err = q.GetCampaign(ctx, old.CampaignID); err != nil {
			return err
		}
		v, err := merge(old, body, "name format width height markup landing_url status", "width height")
		if err != nil {
			return err
		}
		if err = validateCreative(v); err != nil {
			return err
		}
		result, err = q.UpdateCreative(ctx, db.UpdateCreativeParams{ID: v.ID, CampaignID: v.CampaignID, Name: v.Name, Format: v.Format, Width: v.Width, Height: v.Height, Markup: v.Markup, LandingUrl: v.LandingUrl, Status: v.Status})
		return err
	})
	return result, err
}

// Creatives returns a bounded keyset page.
func (s *Service) Creatives(ctx context.Context, parent string, query url.Values) (Page[Creative], error) {
	if !validUUID(parent) {
		return Page[Creative]{}, invalid("campaign_id", "must be a UUID")
	}
	p, err := parsePage(query, "creatives"+"|"+parent)
	if err != nil {
		return Page[Creative]{}, err
	}
	var rows []Creative
	err = s.repo.Read(ctx, func(ctx context.Context, q *db.Queries) error {
		if _, err := q.GetCampaign(ctx, parent); err != nil {
			return err
		}
		var err error
		rows, err = q.ListCreatives(ctx, db.ListCreativesParams{HasCursor: p.HasCursor, CursorTime: p.Time, CursorID: p.ID, PageLimit: p.Limit + 1, CampaignID: parent})
		return err
	})
	if err != nil {
		return Page[Creative]{}, err
	}
	return paginate(rows, p, func(v Creative) (time.Time, string) { return v.CreatedAt, v.ID }), nil
}

// DeleteCreative removes a record; referenced bidders remain protected by foreign keys.
func (s *Service) DeleteCreative(ctx context.Context, id string) error {
	if !validUUID(id) {
		return invalid("id", "invalid uuid")
	}
	return s.repo.Write(ctx, func(ctx context.Context, q *db.Queries) error {
		row, err := q.GetCreative(ctx, id)
		if err != nil {
			return err
		}
		if _, err = q.GetCampaign(ctx, row.CampaignID); err != nil {
			return err
		}
		count, err := q.DeleteCreative(ctx, id)
		if err != nil {
			return err
		}
		if count == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ArchiveAdvertiser retains the record and releases its active-name uniqueness.
func (s *Service) ArchiveAdvertiser(ctx context.Context, id string) error {
	_, err := s.PatchAdvertiser(ctx, id, map[string]json.RawMessage{"status": json.RawMessage(`"archived"`)})
	return err
}
