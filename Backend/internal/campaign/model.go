// Package campaign implements the campaign administration domain and REST API.
package campaign

import (
	"time"

	db "pulsebid/gen/campaigndb"
	"pulsebid/internal/platform/money"
)

// Advertiser is the advertiser domain record.
type Advertiser = db.Advertiser

// Bidder is an administratively registered demand partner.
type Bidder = db.Bidder

// Creative is an ad associated with one immutable parent campaign.
type Creative = db.Creative

// Targeting is the full set of campaign targeting restrictions.
type Targeting = db.TargetingRule

// Campaign exposes exact decimal money while storage uses integer micros.
type Campaign struct {
	ID           string      `json:"id"`
	AdvertiserID string      `json:"advertiser_id"`
	BidderID     string      `json:"bidder_id"`
	Name         string      `json:"name"`
	Status       string      `json:"status"`
	DailyBudget  money.Money `json:"daily_budget"`
	MaxBidCPM    money.Money `json:"max_bid_cpm"`
	PacingMode   string      `json:"pacing_mode"`
	StartAt      *time.Time  `json:"start_at"`
	EndAt        *time.Time  `json:"end_at"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

func campaignFromDB(c db.Campaign) Campaign {
	return Campaign{ID: c.ID, AdvertiserID: c.AdvertiserID, BidderID: c.BidderID, Name: c.Name, Status: c.Status, DailyBudget: money.Money(c.DailyBudgetMicros), MaxBidCPM: money.Money(c.MaxBidCpmMicros), PacingMode: c.PacingMode, StartAt: c.StartAt, EndAt: c.EndAt, CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC()}
}

// Page is the keyset-paginated response envelope. Empty data is always [].
type Page[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

type domainError string

func (e domainError) Error() string { return string(e) }

// Domain sentinels are immutable and preserved through wrapping.
const (
	ErrNotFound    domainError = "resource not found"
	ErrConflict    domainError = "resource conflict"
	ErrValidation  domainError = "validation failed"
	ErrUnavailable domainError = "database unavailable"
)

// ValidationError identifies one invalid public input field.
type ValidationError struct{ Field, Issue string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Issue }

// Unwrap preserves validation classification.
func (e *ValidationError) Unwrap() error { return ErrValidation }
func invalid(field, issue string) error  { return &ValidationError{Field: field, Issue: issue} }
