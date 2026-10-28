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

	"github.com/pp-sem7-team/backend/internal/db"
)

const analysisRequestMessageType = "analysis.request"

var (
	ErrBranchNotFound = errors.New("branch not found")
	ErrBranchClosed   = errors.New("branch is closed")
	ErrChangeNotFound = errors.New("change not found")
	ErrEmptyRawDiff   = errors.New("raw diff is empty")
)

type Querier interface {
	ApplyAnalysisResult(ctx context.Context, arg db.ApplyAnalysisResultParams) (int64, error)
	BranchExists(ctx context.Context, arg db.BranchExistsParams) (bool, error)
	CountChangesByBranch(ctx context.Context, arg db.CountChangesByBranchParams) (int64, error)
	CreateAnalysisResult(ctx context.Context, arg db.CreateAnalysisResultParams) (db.CreateAnalysisResultRow, error)
	CreateChange(ctx context.Context, arg db.CreateChangeParams) (db.CreateChangeRow, error)
	CreateOutboxMessage(ctx context.Context, arg db.CreateOutboxMessageParams) error
	FailStalePendingAnalyses(ctx context.Context, timeoutMs int64) (int64, error)
	GetChange(ctx context.Context, arg db.GetChangeParams) (db.GetChangeRow, error)
	ListChangesByBranch(ctx context.Context, arg db.ListChangesByBranchParams) ([]db.ListChangesByBranchRow, error)
	LockBranchForChange(ctx context.Context, arg db.LockBranchForChangeParams) (db.LockBranchForChangeRow, error)
	WithTx(tx pgx.Tx) *db.Queries
}

type Service struct {
	database *db.DB
	querier  Querier
	log      *slog.Logger
}

func NewService(database *db.DB, querier Querier, log *slog.Logger) *Service {
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
	outboxID := uuid.New()

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
		ID:       analysisID,
		ChangeID: changeID,
	})
	if err != nil {
		s.log.Error("failed to create analysis result",
			"error", err,
			"change_id", changeID,
			"analysis_id", analysisID,
		)
		return Change{}, err
	}

	// TODO: create a pending commit here when the commit flow is implemented.
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
		ID:          outboxID,
		MessageType: analysisRequestMessageType,
		ResourceID:  analysisID,
		Payload:     payload,
	})
	if err != nil {
		s.log.Error("failed to create analysis outbox message",
			"error", err,
			"analysis_id", analysisID,
			"outbox_id", outboxID,
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
			Status:            AnalysisStatus(analysisRow.Status),
			CreatedAt:         analysisRow.CreatedAt,
			UpdatedAt:         analysisRow.UpdatedAt,
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
			Status:            AnalysisStatus(row.AnalysisStatus),
			CreatedAt:         row.AnalysisCreatedAt,
			UpdatedAt:         row.AnalysisUpdatedAt,
		},
		CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Service) List(
	ctx context.Context,
	userID, branchID uuid.UUID,
	limit, offset int32,
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
				Status:            AnalysisStatus(row.AnalysisStatus),
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
	status AnalysisStatus,
	structuredChanges json.RawMessage,
) (bool, error) {
	rowsAffected, err := s.querier.ApplyAnalysisResult(ctx, db.ApplyAnalysisResultParams{
		StructuredChanges: structuredChanges,
		Status:            db.GenerationStatus(status),
		AnalysisID:        analysisID,
		MessageID:         messageID,
	})
	if err != nil {
		s.log.Error("failed to apply analysis result",
			"error", err,
			"analysis_id", analysisID,
			"message_id", messageID,
		)
		return false, err
	}

	return rowsAffected > 0, nil
}

func (s *Service) failStaleAnalyses(ctx context.Context, timeout time.Duration) {
	rowsAffected, err := s.querier.FailStalePendingAnalyses(ctx, timeout.Milliseconds())
	if err != nil {
		s.log.Error("failed to fail stale analyses", "error", err)
		return
	}

	if rowsAffected > 0 {
		s.log.Info("marked stale analyses as failed", "count", rowsAffected)
	}
}
