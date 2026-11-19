package changes

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/pp-sem7-team/backend/internal/commit"
	"github.com/pp-sem7-team/backend/internal/generation"
	"github.com/pp-sem7-team/backend/internal/postgres"
	"github.com/pp-sem7-team/backend/internal/postgres/db"
)

const analysisRequestMessageType = "analysis.request"

var (
	ErrBranchNotFound = errors.New("branch not found")
	ErrBranchClosed   = errors.New("branch is closed")
	ErrChangeNotFound = errors.New("change not found")
	ErrEmptyRawDiff   = errors.New("raw diff is empty")
)

type Querier interface {
	BranchExists(ctx context.Context, arg db.BranchExistsParams) (bool, error)
	CountChangesByBranch(ctx context.Context, arg db.CountChangesByBranchParams) (int, error)
	CreateAnalysisResult(ctx context.Context, arg db.CreateAnalysisResultParams) (db.CreateAnalysisResultRow, error)
	CreateChange(ctx context.Context, arg db.CreateChangeParams) (db.CreateChangeRow, error)
	CreateOutboxMessage(ctx context.Context, arg db.CreateOutboxMessageParams) error
	FailStalePendingAnalyses(ctx context.Context, cutoff time.Time) (int, error)
	GetChange(ctx context.Context, arg db.GetChangeParams) (db.GetChangeRow, error)
	ListChangesByBranch(ctx context.Context, arg db.ListChangesByBranchParams) ([]db.ListChangesByBranchRow, error)
	LockBranchForChange(ctx context.Context, arg db.LockBranchForChangeParams) (db.LockBranchForChangeRow, error)
	WithTx(tx pgx.Tx) *db.Queries
}

type Service struct {
	database *postgres.DB
	querier  Querier
	log      *slog.Logger
}

func NewService(database *postgres.DB, querier Querier, log *slog.Logger) *Service {
	return &Service{
		database: database,
		querier:  querier,
		log:      log,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID, branchID uuid.UUID,
	rawDiff string,
) (Change, error) {
	if strings.TrimSpace(rawDiff) == "" {
		return Change{}, ErrEmptyRawDiff
	}

	tx, err := s.database.Pool.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin create change transaction",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return Change{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	querier := s.querier.WithTx(tx)

	branch, err := querier.LockBranchForChange(ctx, db.LockBranchForChangeParams{
		BranchID: branchID,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Change{}, ErrBranchNotFound
		}

		s.log.Error("failed to lock branch for change",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return Change{}, err
	}

	if branch.Status != db.BranchStatusActive {
		return Change{}, ErrBranchClosed
	}

	changeID := uuid.New()
	analysisID := uuid.New()
	commitID := uuid.New()
	analysisMessageID := uuid.New()
	commitMessageID := uuid.New()

	changeRow, err := querier.CreateChange(ctx, db.CreateChangeParams{
		ID:       changeID,
		BranchID: branchID,
		RawDiff:  rawDiff,
	})
	if err != nil {
		s.log.Error("failed to create change",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
			"change_id", changeID,
		)
		return Change{}, err
	}

	analysisRow, err := querier.CreateAnalysisResult(ctx, db.CreateAnalysisResultParams{
		ID:        analysisID,
		ChangeID:  changeID,
		MessageID: analysisMessageID,
	})
	if err != nil {
		s.log.Error("failed to create analysis result",
			"error", err,
			"change_id", changeID,
			"analysis_id", analysisID,
		)
		return Change{}, err
	}

	commitRow, err := querier.CreateCommit(ctx, db.CreateCommitParams{
		ID:        commitID,
		ChangeID:  changeID,
		MessageID: commitMessageID,
	})
	if err != nil {
		s.log.Error("failed to create commit",
			"error", err,
			"change_id", changeID,
		)
		return Change{}, err
	}

	payload, err := json.Marshal(analysisRequestPayload{
		AnalysisID: analysisID,
		RawDiff:    rawDiff,
	})
	if err != nil {
		s.log.Error("failed to marshal analysis request",
			"error", err,
			"analysis_id", analysisID,
		)
		return Change{}, err
	}

	err = querier.CreateOutboxMessage(ctx, db.CreateOutboxMessageParams{
		ID:          analysisMessageID,
		MessageType: analysisRequestMessageType,
		Payload:     payload,
	})
	if err != nil {
		s.log.Error("failed to create analysis outbox message",
			"error", err,
			"analysis_id", analysisID,
			"outbox_id", analysisMessageID,
		)
		return Change{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.Error("failed to commit create change transaction",
			"error", err,
			"change_id", changeID,
		)
		return Change{}, err
	}

	return Change{
		ID:             changeRow.ID,
		BranchID:       changeRow.BranchID,
		SequenceNumber: changeRow.SequenceNumber,
		Analysis: AnalysisResult{
			ID:                analysisRow.ID,
			StructuredChanges: json.RawMessage(analysisRow.StructuredChanges),
			Status:            generation.Status(analysisRow.Status),
			CreatedAt:         analysisRow.CreatedAt,
			UpdatedAt:         analysisRow.UpdatedAt,
		},
		Commit: &commit.Commit{
			ID:        commitRow.ID,
			Status:    generation.Status(commitRow.Status),
			CreatedAt: commitRow.CreatedAt,
			UpdatedAt: commitRow.UpdatedAt,
		},
		CreatedAt: changeRow.CreatedAt,
	}, nil
}

func (s *Service) Get(
	ctx context.Context,
	userID, changeID uuid.UUID,
) (Change, error) {
	row, err := s.querier.GetChange(ctx, db.GetChangeParams{
		ID:     changeID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Change{}, ErrChangeNotFound
		}

		s.log.Error("failed to get change",
			"error", err,
			"user_id", userID,
			"change_id", changeID,
		)
		return Change{}, err
	}

	return Change{
		ID:             row.ID,
		BranchID:       row.BranchID,
		SequenceNumber: row.SequenceNumber,
		Analysis: AnalysisResult{
			ID:                row.AnalysisID,
			StructuredChanges: json.RawMessage(row.StructuredChanges),
			Status:            generation.Status(row.AnalysisStatus),
			CreatedAt:         row.AnalysisCreatedAt,
			UpdatedAt:         row.AnalysisUpdatedAt,
		},
		CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Service) List(
	ctx context.Context,
	userID, branchID uuid.UUID,
	limit, offset int,
) (ChangeList, error) {
	exists, err := s.querier.BranchExists(ctx, db.BranchExistsParams{
		BranchID: branchID,
		UserID:   userID,
	})
	if err != nil {
		s.log.Error("failed to check branch existence",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return ChangeList{}, err
	}
	if !exists {
		return ChangeList{}, ErrBranchNotFound
	}

	rows, err := s.querier.ListChangesByBranch(ctx, db.ListChangesByBranchParams{
		BranchID:   branchID,
		UserID:     userID,
		PageLimit:  limit,
		PageOffset: offset,
	})
	if err != nil {
		s.log.Error("failed to list changes",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return ChangeList{}, err
	}

	total, err := s.querier.CountChangesByBranch(ctx, db.CountChangesByBranchParams{
		BranchID: branchID,
		UserID:   userID,
	})
	if err != nil {
		s.log.Error("failed to count changes",
			"error", err,
			"user_id", userID,
			"branch_id", branchID,
		)
		return ChangeList{}, err
	}

	items := make([]Change, 0, len(rows))
	for _, row := range rows {
		items = append(items, Change{
			ID:             row.ID,
			BranchID:       row.BranchID,
			SequenceNumber: row.SequenceNumber,
			Analysis: AnalysisResult{
				ID:                row.AnalysisID,
				StructuredChanges: json.RawMessage(row.StructuredChanges),
				Status:            generation.Status(row.AnalysisStatus),
				CreatedAt:         row.AnalysisCreatedAt,
				UpdatedAt:         row.AnalysisUpdatedAt,
			},
			CreatedAt: row.CreatedAt,
		})
	}

	return ChangeList{
		Items:  items,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}, nil
}

func (s *Service) ApplyAnalysisResult(
	ctx context.Context,
	analysisID, messageID uuid.UUID,
	status generation.Status,
	structuredChanges json.RawMessage,
) (bool, error) {
	tx, err := s.database.Pool.Begin(ctx)
	if err != nil {
		s.log.Error("failed to begin analysis result transaction",
			"error", err,
			"analysis_id", analysisID,
			"message_id", messageID,
		)
		return false, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := s.querier.WithTx(tx)
	changeID, err := queries.ApplyAnalysisResult(ctx, db.ApplyAnalysisResultParams{
		StructuredChanges: structuredChanges,
		Status:            db.GenerationStatus(status),
		AnalysisID:        analysisID,
		MessageID:         messageID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		s.log.Error("failed to update analysis result",
			"error", err,
			"analysis_id", analysisID,
			"message_id", messageID,
		)
		return false, err
	}

	if status == generation.Failed {
		if err := queries.FailPendingCommitByChange(ctx, changeID); err != nil {
			s.log.Error("failed to fail pending commit",
				"error", err,
				"analysis_id", analysisID,
				"message_id", messageID,
			)
			return false, err
		}
	} else {
		pending, err := queries.GetPendingCommitByChange(ctx, changeID)
		if err != nil {
			s.log.Error("failed to get pending commit",
				"error", err,
				"analysis_id", analysisID,
				"message_id", messageID,
			)
			return false, err
		}
		if err := commit.EnqueueRequest(ctx, queries, pending.ID, pending.MessageID, structuredChanges); err != nil {
			s.log.Error("failed to enqueue commit request",
				"error", err,
				"analysis_id", analysisID,
				"message_id", messageID,
			)
			return false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.Error("failed to commit analysis result transaction",
			"error", err,
			"analysis_id", analysisID,
			"message_id", messageID,
		)
		return false, err
	}

	return true, nil
}
