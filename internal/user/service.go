package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
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
	q Querier
}

func NewService(q Querier) *Service {
	return &Service{
		q: q,
	}
}

func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (User, error) {
	row, err := s.q.GetUser(ctx, userID)
	if err != nil {
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
		return err
	}

	err = security.CheckPassword(currentPassword, passwordHash)
	if err != nil {
		if errors.Is(err, security.ErrInvalidPassword) {
			return ErrInvalidPassword
		}

		return err
	}

	newPasswordHash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}

	rowsAffected, err := s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		UserID:       userID,
		PasswordHash: newPasswordHash,
	})
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}
