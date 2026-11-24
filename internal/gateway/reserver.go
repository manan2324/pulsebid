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

// Reserve accepts a reservation without contacting Redis.
func (s StubReserver) Reserve(_ context.Context, _ budget.CampaignBudget, _ int64, _ time.Time) (budget.Result, error) {
	return budget.Reserved, nil
}

// Release has no persisted reservation to release.
func (s StubReserver) Release(_ context.Context, _ string, _ int64, _ time.Time) error {
	return nil
}

// Spent returns zero because this test implementation does not persist spend.
func (s StubReserver) Spent(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

// Reset has no persisted budget to reset.
func (s StubReserver) Reset(_ context.Context, _ string, _ time.Time) error {
	return nil
}
