package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pp-sem7-team/backend/internal/config"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.PostgresConfig, logger *slog.Logger) (*DB, error) {
	dsn := buildDSN(cfg)

	logger.Info("creating PostgreSQL connection pool")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("failed to create PostgreSQL connection pool",
			"error", err,
		)
		return nil, err
	}

	logger.Info("PostgreSQL connection pool created")

	return &DB{
		Pool: pool,
	}, nil
}
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

func (db *DB) Close() {
	db.Pool.Close()
}

func buildDSN(c config.PostgresConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
	)
}
