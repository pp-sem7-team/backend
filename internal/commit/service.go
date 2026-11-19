package commit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/generation"
	"github.com/pp-sem7-team/backend/internal/postgres/db"
)

const commitRequestMessageType = "commit.request"

type Service struct {
	querier *db.Queries
	log     *slog.Logger
}

func NewService(querier *db.Queries, log *slog.Logger) *Service {
	return &Service{
		querier: querier,
		log:     log,
	}
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
