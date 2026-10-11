package cleanup

import (
	"context"
	"log/slog"
	"time"
)

const refreshTokensCleanupInterval = time.Minute

type Querier interface {
	DeleteExpiredRefreshTokens(ctx context.Context) (int64, error)
}

func RefreshTokens(ctx context.Context, q Querier, log *slog.Logger) {
	ticker := time.NewTicker(refreshTokensCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := q.DeleteExpiredRefreshTokens(ctx)
			if err != nil {
				log.Error("failed to clean up expired refresh tokens", "error", err)
				continue
			}

			if deleted > 0 {
				log.Info("expired refresh tokens cleaned up", "deleted", deleted)
			}
		}
	}
}
