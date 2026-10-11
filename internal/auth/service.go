package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/security"
)

var (
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type Querier interface {
	CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) error
	CreateUser(ctx context.Context, arg db.CreateUserParams) error
	DeleteRefreshTokenByHash(ctx context.Context, tokenHash string) error
	GetUserCredentialsByEmail(ctx context.Context, email string) (db.GetUserCredentialsByEmailRow, error)
	GetUserIDByRefreshTokenHash(ctx context.Context, tokenHash string) (uuid.UUID, error)
	RotateRefreshToken(ctx context.Context, arg db.RotateRefreshTokenParams) (int64, error)
}

type TokenManager interface {
	GenerateAccessToken(userID uuid.UUID) (string, error)
	GenerateRefreshToken() (string, error)
	HashRefreshToken(token string) string
}

type Service struct {
	q            Querier
	tokenManager TokenManager
	log          *slog.Logger
}

func NewService(q Querier, tokens TokenManager, log *slog.Logger) *Service {
	return &Service{
		q:            q,
		tokenManager: tokens,
		log:          log,
	}
}

func (s *Service) Register(
	ctx context.Context,
	email string,
	password string,
) (TokenPair, error) {
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		s.log.Error("failed to hash password", "error", err)
		return TokenPair{}, err
	}

	userID := uuid.New()

	err = s.q.CreateUser(ctx, db.CreateUserParams{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "users_email_key" {
			return TokenPair{}, ErrEmailAlreadyExists
		}

		s.log.Error("failed to create user", "error", err)
		return TokenPair{}, err
	}

	return s.issueTokens(ctx, userID)
}

func (s *Service) Login(
	ctx context.Context,
	email string,
	password string,
) (TokenPair, error) {
	creds, err := s.q.GetUserCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenPair{}, ErrInvalidCredentials
		}

		s.log.Error("failed to get user credentials", "error", err)
		return TokenPair{}, err
	}

	err = security.CheckPassword(password, creds.PasswordHash)
	if err != nil {
		if errors.Is(err, security.ErrInvalidPassword) {
			return TokenPair{}, ErrInvalidCredentials
		}

		s.log.Error("failed to check password", "error", err)
		return TokenPair{}, err
	}

	return s.issueTokens(ctx, creds.ID)
}

func (s *Service) Refresh(
	ctx context.Context,
	refreshToken string,
) (TokenPair, error) {
	oldTokenHash := s.tokenManager.HashRefreshToken(refreshToken)

	userID, err := s.q.GetUserIDByRefreshTokenHash(ctx, oldTokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenPair{}, ErrInvalidRefreshToken
		}

		s.log.Error("failed to get user by refresh token", "error", err)
		return TokenPair{}, err
	}

	accessToken, err := s.tokenManager.GenerateAccessToken(userID)
	if err != nil {
		s.log.Error("failed to generate access token", "user_id", userID, "error", err)
		return TokenPair{}, err
	}

	newRefreshToken, err := s.tokenManager.GenerateRefreshToken()
	if err != nil {
		s.log.Error("failed to generate refresh token", "user_id", userID, "error", err)
		return TokenPair{}, err
	}

	newTokenHash := s.tokenManager.HashRefreshToken(newRefreshToken)

	rowsAffected, err := s.q.RotateRefreshToken(ctx, db.RotateRefreshTokenParams{
		OldTokenHash: oldTokenHash,
		NewTokenHash: newTokenHash,
		ExpiresAt:    time.Now().Add(refreshTokenTTL),
	})
	if err != nil {
		s.log.Error("failed to rotate refresh token", "user_id", userID, "error", err)
		return TokenPair{}, err
	}

	if rowsAffected == 0 {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := s.tokenManager.HashRefreshToken(refreshToken)

	if err := s.q.DeleteRefreshTokenByHash(ctx, tokenHash); err != nil {
		s.log.Error("failed to delete refresh token", "error", err)
		return err
	}

	return nil
}

func (s *Service) issueTokens(
	ctx context.Context,
	userID uuid.UUID,
) (TokenPair, error) {
	accessToken, err := s.tokenManager.GenerateAccessToken(userID)
	if err != nil {
		s.log.Error("failed to generate access token", "user_id", userID, "error", err)
		return TokenPair{}, err
	}

	refreshToken, err := s.tokenManager.GenerateRefreshToken()
	if err != nil {
		s.log.Error("failed to generate refresh token", "user_id", userID, "error", err)
		return TokenPair{}, err
	}

	err = s.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: s.tokenManager.HashRefreshToken(refreshToken),
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	})
	if err != nil {
		s.log.Error("failed to save refresh token", "user_id", userID, "error", err)
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
