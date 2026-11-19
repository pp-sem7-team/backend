package commit

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pp-sem7-team/backend/internal/generation"
	"github.com/pp-sem7-team/backend/internal/postgres"
	"github.com/pp-sem7-team/backend/internal/postgres/db"
)

const commitRequestMessageType = "commit.request"

var (
	ErrCommitNotFound  = errors.New("commit not found")
	ErrAnalysisNotDone = errors.New("analysis is not done")
	ErrCommitPending   = errors.New("commit generation is pending")
	ErrCommitNotDone   = errors.New("commit generation is not done")
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

func (s *Service) RegenerateCommit(ctx context.Context, userID, changeID uuid.UUID) (*Commit, error) {
	tx, err := s.database.Pool.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin commit regeneration transaction",
			"error", err,
			"change_id", changeID,
		)
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := s.querier.WithTx(tx)
	row, err := queries.LockCommitByChange(ctx, db.LockCommitByChangeParams{
		ChangeID: changeID,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCommitNotFound
		}

		s.log.Error("failed to lock commit for regeneration",
			"error", err,
			"change_id", changeID,
		)
		return nil, err
	}

	if row.AnalysisStatus != db.GenerationStatusDone {
		return nil, ErrAnalysisNotDone
	}
	if row.Status == db.GenerationStatusPending {
		return nil, ErrCommitPending
	}

	messageID := uuid.New()
	commitRow, err := queries.ResetCommit(ctx, db.ResetCommitParams{
		CommitID:  row.ID,
		MessageID: messageID,
	})
	if err != nil {
		s.log.Error("failed to reset commit for regeneration",
			"error", err,
			"commit_id", row.ID,
		)
		return nil, err
	}

	if err := EnqueueRequest(ctx, queries, row.ID, messageID, row.StructuredChanges); err != nil {
		s.log.Error("failed to enqueue commit regeneration request",
			"error", err,
			"commit_id", row.ID,
			"message_id", messageID,
		)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.Error("failed to commit regeneration transaction",
			"error", err,
			"commit_id", row.ID,
		)
		return nil, err
	}

	return &Commit{
		ID:        commitRow.ID,
		Status:    generation.Status(commitRow.Status),
		CreatedAt: commitRow.CreatedAt,
		UpdatedAt: commitRow.UpdatedAt,
	}, nil
}

func (s *Service) EditCommit(
	ctx context.Context,
	userID, changeID uuid.UUID,
	commitType CommitType,
	scope *string,
	message string,
) (*Commit, error) {
	tx, err := s.database.Pool.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin commit edit transaction",
			"error", err,
			"change_id", changeID,
		)
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := s.querier.WithTx(tx)
	row, err := queries.LockCommitByChange(ctx, db.LockCommitByChangeParams{
		ChangeID: changeID,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCommitNotFound
		}
		s.log.Error("failed to lock commit for editing",
			"error", err,
			"change_id", changeID,
		)
		return nil, err
	}

	if row.Status != db.GenerationStatusDone {
		return nil, ErrCommitNotDone
	}

	edited, err := queries.UpdateCommitMessage(ctx, db.UpdateCommitMessageParams{
		CommitID: row.ID,
		Type:     db.CommitType(commitType),
		Scope:    scope,
		Message:  message,
	})
	if err != nil {
		s.log.Error("failed to update commit message",
			"error", err,
			"commit_id", row.ID,
		)
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		s.log.Error("failed to commit edit transaction",
			"error", err,
			"commit_id", row.ID,
		)
		return nil, err
	}

	return &Commit{
		ID:             edited.ID,
		Type:           (*CommitType)(edited.Type),
		Scope:          edited.Scope,
		Message:        edited.Message,
		IsEditedByUser: edited.IsEditedByUser,
		Status:         generation.Status(edited.Status),
		CreatedAt:      edited.CreatedAt,
		UpdatedAt:      edited.UpdatedAt,
	}, nil
}

func (s *Service) ApplyCommitResult(
	ctx context.Context,
	commitID, messageID uuid.UUID,
	status generation.Status,
	commitType *CommitType,
	scope, message *string,
) (bool, error) {
	count, err := s.querier.ApplyCommitResult(ctx, db.ApplyCommitResultParams{
		CommitID:  commitID,
		MessageID: messageID,
		Status:    db.GenerationStatus(status),
		Type:      (*db.CommitType)(commitType),
		Scope:     scope,
		Message:   message,
	})
	if err != nil {
		s.log.Error("failed to apply commit result",
			"error", err,
			"commit_id", commitID,
			"message_id", messageID,
		)
		return false, err
	}
	return count > 0, nil
}

func EnqueueRequest(ctx context.Context, queries *db.Queries, commitID, messageID uuid.UUID, structuredChanges json.RawMessage) error {
	payload, err := json.Marshal(commitRequestPayload{
		CommitID:          commitID,
		StructuredChanges: structuredChanges,
	})
	if err != nil {
		return err
	}

	return queries.CreateOutboxMessage(ctx, db.CreateOutboxMessageParams{
		ID:          messageID,
		MessageType: commitRequestMessageType,
		Payload:     payload,
	})
}
