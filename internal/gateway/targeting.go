package gateway

import (
	"sort"
	"time"

	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
)

// CompiledCampaign is an optimized, read-only in-memory representation of a campaign.
type CompiledCampaign struct {
	ID                string
	AdvertiserID      string
	BidderID          string
	Name              string
	DailyBudgetMicros int64
	MaxBidCPMMicros   int64
	PacingMode        catalogv1.PacingMode
	StartAt           time.Time
	EndAt             time.Time
	Targeting         CompiledTargeting
	Creatives         []*catalogv1.Creative
}

// CompiledTargeting holds pre-parsed targeting constraints for O(1) matching.
type CompiledTargeting struct {
	Countries    map[string]struct{}
	DeviceTypes  map[string]struct{}
	PlacementIDs map[string]struct{}
	HoursOfDay   map[int]struct{}
	Location     *time.Location
}

// Candidate pairs an eligible campaign with its matching creative.
type Candidate struct {
	Campaign *CompiledCampaign
	Creative *catalogv1.Creative
}

// CompileCampaign prepares a protobuf campaign for fast request evaluation.
func CompileCampaign(c *catalogv1.Campaign) *CompiledCampaign {
	cc := &CompiledCampaign{
		ID:                c.GetId(),
		AdvertiserID:      c.GetAdvertiserId(),
		BidderID:          c.GetBidderId(),
		Name:              c.GetName(),
		DailyBudgetMicros: c.GetDailyBudgetMicros(),
		MaxBidCPMMicros:   c.GetMaxBidCpmMicros(),
		PacingMode:        c.GetPacingMode(),
	}

	if c.GetStartAt() != nil {
		cc.StartAt = c.GetStartAt().AsTime().UTC()
	}
	if c.GetEndAt() != nil {
		cc.EndAt = c.GetEndAt().AsTime().UTC()
	}

	// Sort creatives by ID ascending
	if len(c.GetCreatives()) > 0 {
		crs := make([]*catalogv1.Creative, len(c.GetCreatives()))
		copy(crs, c.GetCreatives())
		sort.Slice(crs, func(i, j int) bool {
			return crs[i].GetId() < crs[j].GetId()
		})
		cc.Creatives = crs
	}

	tgt := c.GetTargeting()
	if tgt != nil {
		if len(tgt.GetCountries()) > 0 {
			cc.Targeting.Countries = make(map[string]struct{}, len(tgt.GetCountries()))
			for _, co := range tgt.GetCountries() {
				cc.Targeting.Countries[co] = struct{}{}
			}
		}
		if len(tgt.GetDeviceTypes()) > 0 {
			cc.Targeting.DeviceTypes = make(map[string]struct{}, len(tgt.GetDeviceTypes()))
			for _, d := range tgt.GetDeviceTypes() {
				cc.Targeting.DeviceTypes[d] = struct{}{}
			}
		}
		if len(tgt.GetPlacementIds()) > 0 {
			cc.Targeting.PlacementIDs = make(map[string]struct{}, len(tgt.GetPlacementIds()))
			for _, p := range tgt.GetPlacementIds() {
				cc.Targeting.PlacementIDs[p] = struct{}{}
			}
		}
		if len(tgt.GetHoursOfDay()) > 0 {
			cc.Targeting.HoursOfDay = make(map[int]struct{}, len(tgt.GetHoursOfDay()))
			for _, h := range tgt.GetHoursOfDay() {
				cc.Targeting.HoursOfDay[int(h)] = struct{}{}
			}
		}
		if tz := tgt.GetTimezone(); tz != "" {
			if loc, err := time.LoadLocation(tz); err == nil {
				cc.Targeting.Location = loc
			}
		}
	}
	if cc.Targeting.Location == nil {
		cc.Targeting.Location = time.UTC
	}

	return cc
}

// EvaluateCampaign checks if a campaign matches the request according to Section 12.1.
func EvaluateCampaign(c *CompiledCampaign, req *BidRequest, now time.Time, isExhausted func(string) bool) (*Candidate, bool) {
	// Rule 1: now within [start_at, end_at)
	if !c.StartAt.IsZero() && now.Before(c.StartAt) {
		return nil, false
	}
	if !c.EndAt.IsZero() && !now.Before(c.EndAt) {
		return nil, false
	}

	// Rule 2: geo.country in countries (empty = pass)
	if len(c.Targeting.Countries) > 0 {
		if req.Geo == nil || req.Geo.Country == "" {
			return nil, false
		}
		if _, ok := c.Targeting.Countries[req.Geo.Country]; !ok {
			return nil, false
		}
	}

	// Rule 3: device.type in device_types (empty = pass)
	if len(c.Targeting.DeviceTypes) > 0 {
		if req.Device == nil || req.Device.Type == "" {
			return nil, false
		}
		if _, ok := c.Targeting.DeviceTypes[req.Device.Type]; !ok {
			return nil, false
		}
	}

	// Rule 4: imp.placement_id in placement_ids (empty = pass)
	if len(c.Targeting.PlacementIDs) > 0 {
		if _, ok := c.Targeting.PlacementIDs[req.Imp.PlacementID]; !ok {
			return nil, false
		}
	}

	// Rule 5: hour(now in campaign timezone) in hours_of_day (empty = pass)
	if len(c.Targeting.HoursOfDay) > 0 {
		loc := c.Targeting.Location
		if loc == nil {
			loc = time.UTC
		}
		localHour := now.In(loc).Hour()
		if _, ok := c.Targeting.HoursOfDay[localHour]; !ok {
			return nil, false
		}
	}

	// Rule 6: Creative exists matching size
	if len(c.Creatives) == 0 {
		return nil, false
	}

	var matchedCreative *catalogv1.Creative
	if req.Imp.W > 0 && req.Imp.H > 0 {
		for _, cr := range c.Creatives {
			if int(cr.GetWidth()) == req.Imp.W && int(cr.GetHeight()) == req.Imp.H {
				matchedCreative = cr
				break
			}
		}
	} else {
		matchedCreative = c.Creatives[0]
	}

	if matchedCreative == nil {
		return nil, false
	}

	// Rule 7: Not in local exhausted cache
	if isExhausted != nil && isExhausted(c.ID) {
		return nil, false
	}

	return &Candidate{
		Campaign: c,
		Creative: matchedCreative,
	}, true
}

// GroupCandidates evaluates all campaigns and groups qualifying candidates by bidder_id.
func GroupCandidates(campaigns []*CompiledCampaign, req *BidRequest, now time.Time, isExhausted func(string) bool) map[string][]Candidate {
	groups := make(map[string][]Candidate)
	for _, c := range campaigns {
		if cand, ok := EvaluateCampaign(c, req, now, isExhausted); ok {
			groups[c.BidderID] = append(groups[c.BidderID], *cand)
		}
	}
	return groups
}
