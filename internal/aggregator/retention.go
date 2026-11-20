package aggregator

import (
	"context"
	"log/slog"
	"time"
)

// RetentionJob runs periodic cleanup of processed_events older than dedup retention (Section 8).
type RetentionJob struct {
	store     Store
	retention time.Duration
	interval  time.Duration
	logger    *slog.Logger
}

// NewRetentionJob creates a new RetentionJob.
func NewRetentionJob(store Store, retention, interval time.Duration, logger *slog.Logger) *RetentionJob {
	if interval <= 0 {
		interval = time.Hour
	}
	if retention <= 0 {
		retention = 168 * time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RetentionJob{
		store:     store,
		retention: retention,
		interval:  interval,
		logger:    logger,
	}
}

// Run executes the periodic cleanup until ctx is canceled.
func (j *RetentionJob) Run(ctx context.Context) error {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.logger.Info("starting deduplication retention worker",
		"retention", j.retention,
		"interval", j.interval,
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-j.retention)
			deleted, err := j.store.DeleteOldProcessedEvents(ctx, cutoff, 10000)
			if err != nil {
				j.logger.Error("failed to clean up expired processed_events", "cutoff", cutoff, "error", err)
			} else if deleted > 0 {
				j.logger.Info("purged expired processed_events rows", "deleted", deleted, "cutoff", cutoff)
			}
		}
	}
}
