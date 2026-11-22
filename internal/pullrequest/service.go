package pullrequest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pp-sem7-team/backend/internal/commit"
	"github.com/pp-sem7-team/backend/internal/generation"
	"github.com/pp-sem7-team/backend/internal/postgres"
	"github.com/pp-sem7-team/backend/internal/postgres/db"
)

const pullRequestRequestMessageType = "pull_request.request"

var (
	ErrNoChanges             = errors.New("branch has no changes")
	ErrBranchAnalysesNotDone = errors.New("not all branch changes were analyzed successfully")
	ErrBranchNotFound        = errors.New("branch not found")
	ErrPullRequestNotFound   = errors.New("pull request not found")
	ErrPullRequestPending    = errors.New("pull request generation is pending")
	ErrPullRequestNotDone    = errors.New("pull request generation is not done")
)

type Service struct {
	database *postgres.DB
	querier  *db.Queries
	log      *slog.Logger
}

func NewService(database *postgres.DB, querier *db.Queries, log *slog.Logger) *Service {
	return &Service{
		database: database,
		querier:  querier,
		log:      log,
	}
}

func (s *Service) Generate(ctx context.Context, userID, branchID uuid.UUID) (PullRequest, error) {
	tx, err := s.database.Pool.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin pull request generation transaction",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := s.querier.WithTx(tx)
	branch, err := queries.LockBranch(ctx, db.LockBranchParams{
		BranchID: branchID,
		UserID:   userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PullRequest{}, ErrBranchNotFound
	}
	if err != nil {
		s.log.Error("failed to lock branch",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	messageID := uuid.New()
	result, err := queries.UpsertPendingPullRequest(ctx, db.UpsertPendingPullRequestParams{
		ID:        uuid.New(),
		BranchID:  branch.ID,
		MessageID: messageID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PullRequest{}, ErrPullRequestPending
	}
	if err != nil {
		s.log.Error("failed to upsert pending pull request",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	changes, err := queries.ListPullRequestChanges(ctx, branch.ID)
	if err != nil {
		s.log.Error("failed to list pull request changes",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}
	if len(changes) == 0 {
		return PullRequest{}, ErrNoChanges
	}

	request := pullRequestRequestPayload{
		PullRequestID: result.ID,
		BranchName:    branch.Name,
		Changes:       make([]changeContext, 0, len(changes)),
	}
	for _, row := range changes {
		if !row.AnalysisDone || bytes.Equal(row.StructuredChanges, []byte("null")) {
			return PullRequest{}, ErrBranchAnalysesNotDone
		}

		change := changeContext{
			SequenceNumber:    row.SequenceNumber,
			StructuredChanges: row.StructuredChanges,
		}
		if row.CommitDone {
			change.Commit = &commitContext{
				Type:    commit.CommitType(*row.CommitType),
				Scope:   row.CommitScope,
				Message: *row.CommitMessage,
			}
		}
		request.Changes = append(request.Changes, change)
	}

	payload, err := json.Marshal(request)
	if err != nil {
		s.log.Error("failed to marshal pull request request",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	err = queries.CreateOutboxMessage(ctx, db.CreateOutboxMessageParams{
		ID:          messageID,
		MessageType: pullRequestRequestMessageType,
		Payload:     payload,
	})
	if err != nil {
		s.log.Error("failed to create pull request outbox message",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.Error("failed to commit pull request generation transaction",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	return toPullRequest(db.GetPullRequestRow(result)), nil
}

func (s *Service) Get(ctx context.Context, userID, branchID uuid.UUID) (PullRequest, error) {
	result, err := s.querier.GetPullRequest(ctx, db.GetPullRequestParams{
		BranchID: branchID,
		UserID:   userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PullRequest{}, ErrPullRequestNotFound
	}

	if err != nil {
		s.log.Error("failed to get pull request",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	return toPullRequest(result), nil
}

func (s *Service) Edit(ctx context.Context, userID, branchID uuid.UUID, title, description string) (PullRequest, error) {
	tx, err := s.database.Pool.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin pull request edit transaction",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := s.querier.WithTx(tx)
	_, err = queries.LockBranch(ctx, db.LockBranchParams{
		BranchID: branchID,
		UserID:   userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PullRequest{}, ErrBranchNotFound
	}
	if err != nil {
		s.log.Error("failed to lock branch",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	exists, err := queries.PullRequestExists(ctx, branchID)
	if err != nil {
		s.log.Error("failed to check pull request existence",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	if !exists {
		return PullRequest{}, ErrPullRequestNotFound
	}

	result, err := queries.EditPullRequest(ctx, db.EditPullRequestParams{
		BranchID:    branchID,
		Title:       title,
		Description: description,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return PullRequest{}, ErrPullRequestNotDone
	}
	if err != nil {
		s.log.Error("failed to edit pull request",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.Error("failed to commit pull request edit transaction",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return PullRequest{}, err
	}

	return toPullRequest(db.GetPullRequestRow(result)), nil
}

func (s *Service) ApplyPullRequestResult(
	ctx context.Context,
	pullRequestID, messageID uuid.UUID,
	status generation.Status,
	title, description *string,
) (bool, error) {
	count, err := s.querier.ApplyPullRequestResult(ctx, db.ApplyPullRequestResultParams{
		PullRequestID: pullRequestID,
		MessageID:     messageID,
		Status:        db.GenerationStatus(status),
		Title:         title,
		Description:   description,
	})
	if err != nil {
		s.log.Error("failed to apply pull request result",
			"error", err,
			"pull_request_id", pullRequestID,
			"message_id", messageID,
		)
		return false, err
	}
	return count > 0, nil
}

func toPullRequest(row db.GetPullRequestRow) PullRequest {
	return PullRequest{
		ID:             row.ID,
		BranchID:       row.BranchID,
		Title:          row.Title,
		Description:    row.Description,
		IsEditedByUser: row.IsEditedByUser,
		Status:         generation.Status(row.Status),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
