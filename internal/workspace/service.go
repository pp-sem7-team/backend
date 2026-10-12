package workspace

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pp-sem7-team/backend/internal/db"
)

var (
	ErrWorkspaceNotFound   = errors.New("workspace not found")
	ErrWorkspaceNameExists = errors.New("workspace name already exists")
)

type Querier interface {
	CreateWorkspace(ctx context.Context, arg db.CreateWorkspaceParams) (db.CreateWorkspaceRow, error)
	ListWorkspaces(ctx context.Context, arg db.ListWorkspacesParams) ([]db.ListWorkspacesRow, error)
	CountWorkspaces(ctx context.Context, userID uuid.UUID) (int64, error)
	GetWorkspace(ctx context.Context, arg db.GetWorkspaceParams) (db.GetWorkspaceRow, error)
	UpdateWorkspace(ctx context.Context, arg db.UpdateWorkspaceParams) (db.UpdateWorkspaceRow, error)
	DeleteWorkspace(ctx context.Context, arg db.DeleteWorkspaceParams) (int64, error)
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

func (s *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
	name string,
) (Workspace, error) {
	row, err := s.q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:     uuid.New(),
		UserID: userID,
		Name:   name,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "workspaces_user_id_name_key" {
			return Workspace{}, ErrWorkspaceNameExists
		}
		s.log.Error("failed to create workspace", "user_id", userID, "error", err)
		return Workspace{}, err
	}

	return Workspace{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) Get(
	ctx context.Context,
	userID uuid.UUID,
	workspaceID uuid.UUID,
) (Workspace, error) {
	row, err := s.q.GetWorkspace(ctx, db.GetWorkspaceParams{
		ID:     workspaceID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrWorkspaceNotFound
		}

		s.log.Error("failed to get workspace", "workspace_id", workspaceID, "user_id", userID, "error", err)
		return Workspace{}, err
	}

	return Workspace{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) List(
	ctx context.Context,
	userID uuid.UUID,
	limit int32,
	offset int32,
) (WorkspaceList, error) {
	rows, err := s.q.ListWorkspaces(ctx, db.ListWorkspacesParams{
		UserID:     userID,
		PageLimit:  limit,
		PageOffset: offset,
	})
	if err != nil {
		s.log.Error("failed to list workspaces", "user_id", userID, "error", err)
		return WorkspaceList{}, err
	}

	total, err := s.q.CountWorkspaces(ctx, userID)
	if err != nil {
		s.log.Error("failed to count workspaces", "user_id", userID, "error", err)
		return WorkspaceList{}, err
	}

	items := make([]Workspace, 0, len(rows))
	for _, row := range rows {
		items = append(items, Workspace{
			ID:        row.ID,
			Name:      row.Name,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}

	return WorkspaceList{
		Items:  items,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}, nil
}

func (s *Service) Rename(
	ctx context.Context,
	userID uuid.UUID,
	workspaceID uuid.UUID,
	name string,
) (Workspace, error) {
	row, err := s.q.UpdateWorkspace(ctx, db.UpdateWorkspaceParams{
		ID:     workspaceID,
		UserID: userID,
		Name:   name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrWorkspaceNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "workspaces_user_id_name_key" {
			return Workspace{}, ErrWorkspaceNameExists
		}

		s.log.Error("failed to rename workspace", "workspace_id", workspaceID, "user_id", userID, "error", err)
		return Workspace{}, err
	}

	return Workspace{
		ID:        row.ID,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) Delete(
	ctx context.Context,
	userID uuid.UUID,
	workspaceID uuid.UUID,
) error {
	rowsAffected, err := s.q.DeleteWorkspace(ctx, db.DeleteWorkspaceParams{
		ID:     workspaceID,
		UserID: userID,
	})
	if err != nil {
		s.log.Error("failed to delete workspace", "workspace_id", workspaceID, "user_id", userID, "error", err)
		return err
	}

	if rowsAffected == 0 {
		return ErrWorkspaceNotFound
	}

	return nil
}
