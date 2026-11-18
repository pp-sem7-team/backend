package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/postgres/db"
)

const (
	pollInterval      = time.Second
	cleanupInterval   = time.Hour
	batchSize         = 10
	maxAttempts       = 5
	initialRetryDelay = 5 * time.Second
)

type Querier interface {
	DeleteExpiredOutboxMessages(ctx context.Context, cutoff time.Time) (int64, error)
	ListUnpublishedOutboxMessages(ctx context.Context, batchSize int) ([]db.ListUnpublishedOutboxMessagesRow, error)
	MarkOutboxMessagePublished(ctx context.Context, id uuid.UUID) error
	MarkOutboxMessageAttemptFailed(ctx context.Context, arg db.MarkOutboxMessageAttemptFailedParams) error
}

type Publisher interface {
	PublishCommand(ctx context.Context, routingKey, messageID string, body []byte) error
}

type Worker struct {
	querier   Querier
	publisher Publisher
	retention time.Duration
	log       *slog.Logger
}

func NewWorker(querier Querier, publisher Publisher, retention time.Duration, log *slog.Logger) *Worker {
	return &Worker{
		querier:   querier,
		publisher: publisher,
		retention: retention,
		log:       log,
	}
}

func (w *Worker) Run(ctx context.Context) {
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()

	cleanupTicker := time.NewTicker(cleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			w.processBatch(ctx)
		case <-cleanupTicker.C:
			w.cleanup(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	messages, err := w.querier.ListUnpublishedOutboxMessages(ctx, batchSize)
	if err != nil {
		w.log.Error("failed to list unpublished outbox messages", "error", err)
		return
	}

	for _, message := range messages {
		if ctx.Err() != nil {
			return
		}

		if err := w.publisher.PublishCommand(
			ctx,
			message.MessageType,
			message.ID.String(),
			message.Payload,
		); err != nil {
			if ctx.Err() != nil {
				return
			}

			w.log.Error("failed to publish outbox message",
				"error", err,
				"outbox_id", message.ID,
				"message_type", message.MessageType,
				"attempt", message.AttemptCount+1,
			)
			w.recordPublishFailure(ctx, message, err)
			continue
		}

		if err := w.querier.MarkOutboxMessagePublished(ctx, message.ID); err != nil {
			w.log.Error("failed to mark outbox message as published",
				"error", err,
				"outbox_id", message.ID,
			)
			continue
		}

		w.log.Debug("outbox message published",
			"outbox_id", message.ID,
			"message_type", message.MessageType,
		)
	}
}

func (w *Worker) recordPublishFailure(ctx context.Context, message db.ListUnpublishedOutboxMessagesRow, publishErr error) {
	attempt := message.AttemptCount + 1
	now := time.Now()
	nextAttemptAt := now
	var failedAt *time.Time
	if attempt >= maxAttempts {
		failedAt = &now
	} else {
		delay := initialRetryDelay * time.Duration(1<<(attempt-1))
		nextAttemptAt = now.Add(delay)
	}

	if err := w.querier.MarkOutboxMessageAttemptFailed(ctx, db.MarkOutboxMessageAttemptFailedParams{
		ID:            message.ID,
		ErrorMessage:  publishErr.Error(),
		NextAttemptAt: nextAttemptAt,
		FailedAt:      failedAt,
	}); err != nil {
		w.log.Error("failed to record outbox publication failure",
			"error", err,
			"outbox_id", message.ID,
		)
		return
	}

	if failedAt != nil {
		w.log.Error("outbox publication attempts exhausted",
			"outbox_id", message.ID,
			"attempt", attempt,
		)
	} else {
		w.log.Info("outbox publication retry scheduled",
			"outbox_id", message.ID,
			"attempt", attempt,
			"next_attempt_at", nextAttemptAt,
		)
	}
}

func (w *Worker) cleanup(ctx context.Context) {
	cutoff := time.Now().Add(-w.retention)
	deleted, err := w.querier.DeleteExpiredOutboxMessages(ctx, cutoff)
	if err != nil {
		w.log.Error("failed to delete expired outbox messages", "error", err)
		return
	}

	if deleted > 0 {
		w.log.Info("deleted expired outbox messages", "count", deleted)
	}
}
