package commit

import (
	"time"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type CommitType string

const (
	CommitTypeFeat     CommitType = "feat"
	CommitTypeFix      CommitType = "fix"
	CommitTypeRefactor CommitType = "refactor"
	CommitTypePerf     CommitType = "perf"
	CommitTypeDocs     CommitType = "docs"
	CommitTypeTest     CommitType = "test"
	CommitTypeStyle    CommitType = "style"
	CommitTypeChore    CommitType = "chore"
)

type Commit struct {
	ID             uuid.UUID
	Type           *CommitType
	Scope          *string
	Message        *string
	IsEditedByUser bool
	Status         generation.Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (t CommitType) IsValid() bool {
	switch t {
	case CommitTypeFeat, CommitTypeFix, CommitTypeRefactor, CommitTypePerf,
		CommitTypeDocs, CommitTypeTest, CommitTypeStyle, CommitTypeChore:
		return true
	default:
		return false
	}
}
