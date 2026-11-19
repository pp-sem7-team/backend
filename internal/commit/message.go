package commit

import (
	"encoding/json"

	"github.com/google/uuid"
)

type commitRequestPayload struct {
	CommitID          uuid.UUID       `json:"commit_id"`
	StructuredChanges json.RawMessage `json:"structured_changes"`
}
