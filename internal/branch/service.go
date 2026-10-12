package branch

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
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrBranchNotFound    = errors.New("branch not found")
	ErrBranchNameExists  = errors.New("branch name already exists")
	ErrEmptyUpdate       = errors.New("no fields to update")
)

type Querier interface {
	CountBranches(ctx context.Context, arg db.CountBranchesParams) (int64, error)
	CreateBranch(ctx context.Context, arg db.CreateBranchParams) (db.Branch, error)
	DeleteBranch(ctx context.Context, arg db.DeleteBranchParams) (int64, error)
	GetBranch(ctx context.Context, arg db.GetBranchParams) (db.Branch, error)
	ListBranches(ctx context.Context, arg db.ListBranchesParams) ([]db.Branch, error)
	UpdateBranch(ctx context.Context, arg db.UpdateBranchParams) (db.Branch, error)
	WorkspaceExists(ctx context.Context, arg db.WorkspaceExistsParams) (bool, error)
}

type Service struct {
	querier Querier
	log     *slog.Logger
}

func NewService(querier Querier, log *slog.Logger) *Service {
	return &Service{
		querier: querier,
		log:     log,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID, workspaceID uuid.UUID,
	name string,
) (Branch, error) {
	row, err := s.querier.CreateBranch(ctx, db.CreateBranchParams{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Name:        name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Branch{}, ErrWorkspaceNotFound
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "branches_workspace_id_name_key" {
			return Branch{}, ErrBranchNameExists
		}

		s.log.Error("failed to create branch",
			"error", err,
			"user_id", userID,
			"workspace_id", workspaceID,
		)
		return Branch{}, err
	}

	return toBranch(row), nil
}

func (s *Service) Get(
	ctx context.Context,
	userID, branchID uuid.UUID,
) (Branch, error) {
	row, err := s.querier.GetBranch(ctx, db.GetBranchParams{
		ID:     branchID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Branch{}, ErrBranchNotFound
		}

		s.log.Error("failed to get branch",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return Branch{}, err
	}

	return toBranch(row), nil
}

func (s *Service) List(
	ctx context.Context,
	userID, workspaceID uuid.UUID,
	limit, offset int32,
	status *BranchStatus,
) (BranchList, error) {
	exists, err := s.querier.WorkspaceExists(ctx, db.WorkspaceExistsParams{
		ID:     workspaceID,
		UserID: userID,
	})
	if err != nil {
		s.log.Error("failed to check workspace existence",
			"error", err,
			"user_id", userID,
			"workspace_id", workspaceID,
		)
		return BranchList{}, err
	}
	if !exists {
		return BranchList{}, ErrWorkspaceNotFound
	}

	var dbStatus *db.BranchStatus
	if status != nil {
		dbStatus = (*db.BranchStatus)(status)
	}

	params := db.ListBranchesParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Status:      dbStatus,
		PageLimit:   limit,
		PageOffset:  offset,
	}

	rows, err := s.querier.ListBranches(ctx, params)
	if err != nil {
		s.log.Error("failed to list branches",
			"error", err,
			"user_id", userID,
			"workspace_id", workspaceID,
		)
		return BranchList{}, err
	}

	total, err := s.querier.CountBranches(ctx, db.CountBranchesParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Status:      dbStatus,
	})
	if err != nil {
		s.log.Error("failed to count branches",
			"error", err,
			"user_id", userID,
			"workspace_id", workspaceID,
		)
		return BranchList{}, err
	}

	items := make([]Branch, 0, len(rows))
	for _, row := range rows {
		items = append(items, toBranch(row))
	}

	return BranchList{
		Items:  items,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}, nil
}

func (s *Service) Update(
	ctx context.Context,
	userID, branchID uuid.UUID,
	name *string,
	status *BranchStatus,
) (Branch, error) {
	if name == nil && status == nil {
		return Branch{}, ErrEmptyUpdate
	}

	var dbStatus *db.BranchStatus
	if status != nil {
		dbStatus = (*db.BranchStatus)(status)
	}

	row, err := s.querier.UpdateBranch(ctx, db.UpdateBranchParams{
		Name:   name,
		Status: dbStatus,
		ID:     branchID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Branch{}, ErrBranchNotFound
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "branches_workspace_id_name_key" {
			return Branch{}, ErrBranchNameExists
		}

		s.log.Error("failed to update branch",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return Branch{}, err
	}

	return toBranch(row), nil
}

func (s *Service) Delete(
	ctx context.Context,
	userID, branchID uuid.UUID,
) error {
	rowsAffected, err := s.querier.DeleteBranch(ctx, db.DeleteBranchParams{
		ID:     branchID,
		UserID: userID,
	})
	if err != nil {
		s.log.Error("failed to delete branch",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return err
	}

	if rowsAffected == 0 {
		return ErrBranchNotFound
	}

	return nil
}

func toBranch(row db.Branch) Branch {
	return Branch{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		Status:      BranchStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
