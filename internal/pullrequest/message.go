package pullrequest

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/commit"
	"github.com/pp-sem7-team/backend/internal/generation"
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

type pullRequestResultMessage struct {
	PullRequestID uuid.UUID         `json:"pull_request_id"`
	Status        generation.Status `json:"status"`
	Title         *string           `json:"title"`
	Description   *string           `json:"description"`
	Error         *generation.Error `json:"error"`
}
