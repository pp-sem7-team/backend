package pullrequest

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/commit"
)

type pullRequestRequestPayload struct {
	PullRequestID uuid.UUID       `json:"pull_request_id"`
	BranchName    string          `json:"branch_name"`
	Changes       []changeContext `json:"changes"`
}

type changeContext struct {
	SequenceNumber    int             `json:"sequence_number"`
	StructuredChanges json.RawMessage `json:"structured_changes"`
	Commit            *commitContext  `json:"commit"`
}

type commitContext struct {
	Type    commit.CommitType `json:"type"`
	Scope   *string           `json:"scope"`
	Message string            `json:"message"`
}
