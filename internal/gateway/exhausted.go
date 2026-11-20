package gateway

import (
	"sync"
	"time"
)

// ExhaustedCache holds campaign IDs that recently failed budget reservation (5s TTL, Section 9.1).
// This is kept in memory only to prevent calling bidders for dead campaigns.
type ExhaustedCache struct {
	mu      sync.RWMutex
	entries map[string]time.Time
	ttl     time.Duration
}

// NewExhaustedCache initializes a new in-memory cache for exhausted campaigns.
func NewExhaustedCache(ttl time.Duration) *ExhaustedCache {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	return &ExhaustedCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// Mark records a campaign as exhausted with a TTL.
func (c *ExhaustedCache) Mark(campaignID string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[campaignID] = now.Add(c.ttl)
}

// IsExhausted checks whether a campaign is currently marked as exhausted.
func (c *ExhaustedCache) IsExhausted(campaignID string, now time.Time) bool {
	c.mu.RLock()
	expiry, exists := c.entries[campaignID]
	c.mu.RUnlock()

	if !exists {
		return false
	}
	if now.After(expiry) {
		c.mu.Lock()
		if curExp, ok := c.entries[campaignID]; ok && now.After(curExp) {
			delete(c.entries, campaignID)
		}
		c.mu.Unlock()
		return false
	}
	return true
}

// Clear clears all exhausted entries (used in tests).
func (c *ExhaustedCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]time.Time)
}
