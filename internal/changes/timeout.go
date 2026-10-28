package changes

import (
	"context"
	"time"
)

const timeoutCheckInterval = 30 * time.Second

func (s *Service) RunTimeoutWorker(ctx context.Context, timeout time.Duration) {
	s.failStaleAnalyses(ctx, timeout)

	ticker := time.NewTicker(timeoutCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.failStaleAnalyses(ctx, timeout)
		}
	}
}
