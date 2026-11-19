package commit

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type commitRequestPayload struct {
	CommitID          uuid.UUID       `json:"commit_id"`
	StructuredChanges json.RawMessage `json:"structured_changes"`
}

type commitResultMessage struct {
	CommitID uuid.UUID         `json:"commit_id"`
	Status   generation.Status `json:"status"`
	Type     *CommitType       `json:"type"`
	Scope    *string           `json:"scope"`
	Message  *string           `json:"message"`
	Error    *generation.Error `json:"error"`
}
