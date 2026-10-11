package user

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/security"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid current password")
)

type Querier interface {
	GetUser(ctx context.Context, userID uuid.UUID) (db.GetUserRow, error)
	GetUserPasswordHash(ctx context.Context, userID uuid.UUID) (string, error)
	UpdateUserPassword(ctx context.Context, arg db.UpdateUserPasswordParams) (int64, error)
}

type Service struct {
	q   Querier
	log *slog.Logger
}

func NewService(q Querier, log *slog.Logger) *Service {
	return &Service{
		q:   q,
		log: log,
	}
}

func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (User, error) {
	row, err := s.q.GetUser(ctx, userID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.Error("failed to get user", "user_id", userID, "error", err)
		}

		return User{}, err
	}

	return User{
		ID:        row.ID,
		Email:     row.Email,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	currentPassword string,
	newPassword string,
) error {
	passwordHash, err := s.q.GetUserPasswordHash(ctx, userID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.Error("failed to get user password hash", "user_id", userID, "error", err)
		}

		return err
	}

	err = security.CheckPassword(currentPassword, passwordHash)
	if err != nil {
		if errors.Is(err, security.ErrInvalidPassword) {
			return ErrInvalidPassword
		}

		s.log.Error("failed to check current password", "user_id", userID, "error", err)
		return err
	}

	newPasswordHash, err := security.HashPassword(newPassword)
	if err != nil {
		s.log.Error("failed to hash new password", "user_id", userID, "error", err)
		return err
	}

	rowsAffected, err := s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		UserID:       userID,
		PasswordHash: newPasswordHash,
	})
	if err != nil {
		s.log.Error("failed to update user password", "user_id", userID, "error", err)
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}
