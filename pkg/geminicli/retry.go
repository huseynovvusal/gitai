package geminicli

import (
	"context"
	"time"
)

// Retryer handles retrying operations with backoff logic
type Retryer struct {
	MaxRetries  int
	Backoff     func(attempt int) time.Duration
	ShouldRetry func(err error) bool
	Logger      Logger
}

// Do executes the given function with retry logic
func (r *Retryer) Do(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * time.Second
			if r.Backoff != nil {
				delay = r.Backoff(attempt)
			}

			if r.Logger != nil {
				r.Logger.InfoWith("Retrying operation", "attempt", attempt, "max_retries", r.MaxRetries)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if r.ShouldRetry != nil && !r.ShouldRetry(err) {
			return err
		}
	}
	return lastErr
}
