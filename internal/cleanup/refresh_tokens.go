package cleanup

import (
	"context"
	"time"
)

const refreshTokensCleanupInterval = time.Minute

type Querier interface {
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
}

func RefreshTokens(ctx context.Context, q Querier) {
	ticker := time.NewTicker(refreshTokensCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = q.DeleteExpiredRefreshTokens(ctx)
		}
	}
}
