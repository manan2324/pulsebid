package gateway

import (
	"context"
	"time"

	"pulsebid/internal/budget"
)

// ReservationResult aliases budget.Result for gateway consumers.
type ReservationResult = budget.Result

const (
	// Reserved indicates budget was successfully held for this impression.
	Reserved = budget.Reserved
	// Exhausted indicates the campaign has reached its daily budget cap.
	Exhausted = budget.Exhausted
	// Paced indicates budget is available today but currently paced out for this window.
	Paced = budget.Paced
)

// Reserver defines atomic budget reservation across gateway replicas.
type Reserver = budget.Reserver

// StubReserver satisfies Reserver for tests when Redis budget enforcement is not wired.
type StubReserver struct{}

func (s StubReserver) Reserve(ctx context.Context, c budget.CampaignBudget, costMicros int64, now time.Time) (budget.Result, error) {
	return budget.Reserved, nil
}

func (s StubReserver) Release(ctx context.Context, campaignID string, costMicros int64, now time.Time) error {
	return nil
}

func (s StubReserver) Spent(ctx context.Context, campaignID string, now time.Time) (int64, error) {
	return 0, nil
}

func (s StubReserver) Reset(ctx context.Context, campaignID string, now time.Time) error {
	return nil
}
