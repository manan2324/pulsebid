package campaign

import (
	"bytes"
	"encoding/json"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Ship IANA zones in the distroless runtime.
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/language"
)

func validUUID(id string) bool { _, err := uuid.Parse(id); return err == nil && len(id) == 36 }
func validSlug(id string) bool {
	if len(id) < 2 || len(id) > 63 {
		return false
	}
	for i, c := range id {
		isLetter := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isHyphen := c == '-' && i > 0
		if !isLetter && !isDigit && !isHyphen {
			return false
		}
	}
	return true
}
func validName(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= 200
}
func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}
func validStatus(status string) bool {
	return slices.Contains([]string{"draft", "active", "paused", "archived"}, status)
}

func validateAdvertiser(a Advertiser) error {
	if !validName(a.Name) {
		return invalid("name", "must contain 1 to 200 characters")
	}
	if a.Status != "active" && a.Status != "archived" {
		return invalid("status", "must be active or archived")
	}
	if a.ContactEmail != nil {
		address, err := mail.ParseAddress(*a.ContactEmail)
		if err != nil || address.Address != *a.ContactEmail {
			return invalid("contact_email", "must be a valid email address or null")
		}
	}
	return nil
}
func validateBidder(b Bidder) error {
	if !validSlug(b.ID) {
		return invalid("id", "must be a lowercase bidder slug of 2 to 63 characters")
	}
	if !validName(b.Name) {
		return invalid("name", "must contain 1 to 200 characters")
	}
	if !validURL(b.EndpointUrl) {
		return invalid("endpoint_url", "must be an absolute http(s) URL without credentials")
	}
	if b.TimeoutMs != nil && (*b.TimeoutMs < 10 || *b.TimeoutMs > 1000) {
		return invalid("timeout_ms", "must be 10 to 1000 or null")
	}
	return nil
}
func validateCampaign(c *Campaign) error {
	if !validUUID(c.AdvertiserID) {
		return invalid("advertiser_id", "must be a UUID")
	}
	if !validSlug(c.BidderID) {
		return invalid("bidder_id", "must be a valid bidder slug")
	}
	if !validName(c.Name) {
		return invalid("name", "must contain 1 to 200 characters")
	}
	if !validStatus(c.Status) {
		return invalid("status", "must be draft, active, paused, or archived")
	}
	if c.DailyBudget <= 0 || c.DailyBudget > 1_000_000_000_000 {
		return invalid("daily_budget", "must be greater than 0 and at most 1000000")
	}
	if c.MaxBidCPM <= 0 || c.MaxBidCPM > 1_000_000_000 {
		return invalid("max_bid_cpm", "must be greater than 0 and at most 1000")
	}
	if c.PacingMode != "asap" && c.PacingMode != "even" {
		return invalid("pacing_mode", "must be asap or even")
	}
	if c.StartAt != nil {
		u := c.StartAt.UTC()
		c.StartAt = &u
	}
	if c.EndAt != nil {
		u := c.EndAt.UTC()
		c.EndAt = &u
	}
	if c.StartAt != nil && c.EndAt != nil && !c.EndAt.After(*c.StartAt) {
		return invalid("end_at", "must be after start_at")
	}
	return nil
}
func validateCreative(c Creative) error {
	if !validName(c.Name) {
		return invalid("name", "must contain 1 to 200 characters")
	}
	if !slices.Contains([]string{"banner", "native", "video"}, c.Format) {
		return invalid("format", "must be banner, native, or video")
	}
	if c.Width != nil && *c.Width <= 0 {
		return invalid("width", "must be positive or null")
	}
	if c.Height != nil && *c.Height <= 0 {
		return invalid("height", "must be positive or null")
	}
	if c.Markup == "" || len(c.Markup) > 64<<10 {
		return invalid("markup", "must contain 1 to 65536 bytes")
	}
	if !validURL(c.LandingUrl) {
		return invalid("landing_url", "must be an absolute http(s) URL without credentials")
	}
	if c.Status != "active" && c.Status != "paused" {
		return invalid("status", "must be active or paused")
	}
	return nil
}

func normalizeTargeting(t *Targeting) error {
	for i, country := range t.Countries {
		country = strings.ToUpper(country)
		region, err := language.ParseRegion(country)
		if len(country) != 2 || err != nil || !region.IsCountry() || region.String() != country {
			return invalid("countries", "must contain ISO alpha-2 country codes")
		}
		t.Countries[i] = country
	}
	for _, device := range t.DeviceTypes {
		if !slices.Contains([]string{"mobile", "desktop", "tablet", "ctv"}, device) {
			return invalid("device_types", "contains an unknown device type")
		}
	}
	for _, placement := range t.PlacementIds {
		if strings.TrimSpace(placement) == "" {
			return invalid("placement_ids", "must not contain empty placements")
		}
	}
	for _, hour := range t.HoursOfDay {
		if hour < 0 || hour > 23 {
			return invalid("hours_of_day", "must be between 0 and 23")
		}
	}
	if t.Timezone == "" || t.Timezone == "Local" {
		return invalid("timezone", "must be a valid IANA timezone")
	}
	if _, err := time.LoadLocation(t.Timezone); err != nil {
		return invalid("timezone", "must be a valid IANA timezone")
	}
	t.Countries = unique(t.Countries)
	t.DeviceTypes = unique(t.DeviceTypes)
	t.PlacementIds = unique(t.PlacementIds)
	if t.HoursOfDay == nil {
		t.HoursOfDay = []int16{}
	}
	slices.Sort(t.HoursOfDay)
	t.HoursOfDay = slices.Compact(t.HoursOfDay)
	return nil
}
func unique(values []string) []string {
	if values == nil {
		return []string{}
	}
	slices.Sort(values)
	return slices.Compact(values)
}

// merge applies only allowed input fields. Null clears nullable fields and is
// rejected for required scalar fields instead of silently becoming a zero value.
func merge[T any](current T, patch map[string]json.RawMessage, allowed, nullable string) (T, error) {
	var result T
	encoded, err := json.Marshal(current)
	if err != nil {
		return result, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &fields); err != nil {
		return result, err
	}
	for key, value := range patch {
		if !slices.Contains(strings.Fields(allowed), key) {
			return result, invalid(key, "unknown or read-only field")
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) && !slices.Contains(strings.Fields(nullable), key) {
			return result, invalid(key, "must not be null")
		}
		fields[key] = value
	}
	encoded, err = json.Marshal(fields)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(encoded, &result); err != nil {
		return result, invalid("body", "invalid field type, timestamp, or money precision")
	}
	return result, nil
}
