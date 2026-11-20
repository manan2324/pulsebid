package gateway

import (
	"pulsebid/internal/events"
)

// EventPublisher defines the contract for emitting auction and interaction events.
type EventPublisher = events.Publisher

// NoopEventPublisher satisfies EventPublisher when Kafka publishing is not wired or in tests.
type NoopEventPublisher = events.NoopPublisher
