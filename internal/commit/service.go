package commit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/postgres/db"
)

const commitRequestMessageType = "commit.request"

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
