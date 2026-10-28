package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/db"
)

const (
	pollInterval    = time.Second
	cleanupInterval = time.Hour
	batchSize       = int32(10)
)

type Querier interface {
	DeletePublishedOutboxMessages(ctx context.Context, retentionMs int64) (int64, error)
	ListUnpublishedOutboxMessages(ctx context.Context, batchSize int32) ([]db.ListUnpublishedOutboxMessagesRow, error)
	MarkOutboxMessagePublished(ctx context.Context, id uuid.UUID) error
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
		if err := w.publisher.PublishCommand(
			ctx,
			message.MessageType,
			message.ID.String(),
			message.Payload,
		); err != nil {
			w.log.Error("failed to publish outbox message",
				"error", err,
				"outbox_id", message.ID,
				"message_type", message.MessageType,
			)
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

func (w *Worker) cleanup(ctx context.Context) {
	deleted, err := w.querier.DeletePublishedOutboxMessages(ctx, w.retention.Milliseconds())
	if err != nil {
		w.log.Error("failed to delete published outbox messages", "error", err)
		return
	}

	if deleted > 0 {
		w.log.Info("deleted published outbox messages", "count", deleted)
	}
}
