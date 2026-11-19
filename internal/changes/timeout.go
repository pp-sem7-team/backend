package changes

import (
	"context"
	"time"
)

const timeoutCheckInterval = 30 * time.Second

func (s *Service) RunTimeoutWorker(ctx context.Context, timeout time.Duration) {
	ticker := time.NewTicker(timeoutCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := s.querier.FailStalePendingAnalyses(ctx, time.Now().Add(-timeout))
			if err != nil {
				s.log.Error("failed to fail stale analyses", "error", err)
				continue
			}
			if count > 0 {
				s.log.Info("marked stale analyses as failed", "count", count)
			}
		}
	}
}
