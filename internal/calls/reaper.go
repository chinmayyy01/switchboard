package calls

import (
	"context"
	"log/slog"
	"time"

	"github.com/chinmay/switchboard/internal/metrics"
)

// StartReaper closes stale open calls in the background. The webhook is the
// primary lifecycle signal; this is the safety net for a crashed agent or a
// missed delivery.
func StartReaper(ctx context.Context, store *Store, interval, staleAfter time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := store.ReapStale(ctx, staleAfter, "")
				if err != nil {
					slog.Error("call reaper", "err", err)
					continue
				}
				if n > 0 {
					metrics.CallsReaped(n)
					slog.Warn("closed stale calls", "count", n, "older_than", staleAfter.String())
				}
			}
		}
	}()
}
