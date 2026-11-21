package pullrequest

import (
	"time"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type PullRequest struct {
	ID             uuid.UUID
	BranchID       uuid.UUID
	Title          *string
	Description    *string
	IsEditedByUser bool
	Status         generation.Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
