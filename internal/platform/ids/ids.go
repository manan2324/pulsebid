// Package ids implements entity and deterministic identifiers.
package ids

import (
	"fmt"
	"github.com/google/uuid"
)

// New creates an entity UUIDv7 without hiding entropy failures.
func New() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate UUIDv7: %w", err)
	}
	return id.String(), nil
}

// V5 deterministically names a value within the caller's UUID namespace.
func V5(namespace, name string) (string, error) {
	ns, err := uuid.Parse(namespace)
	if err != nil {
		return "", fmt.Errorf("parse UUID namespace: %w", err)
	}
	return uuid.NewSHA1(ns, []byte(name)).String(), nil
}
