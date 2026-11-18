package commit

import (
	"time"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type Commit struct {
	ID             uuid.UUID
	Type           *string
	Scope          *string
	Message        *string
	IsEditedByUser bool
	Status         generation.Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
