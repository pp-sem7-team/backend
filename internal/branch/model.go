package branch

import (
	"time"

	"github.com/google/uuid"
)

type BranchStatus string

const (
	BranchStatusActive BranchStatus = "active"
	BranchStatusClosed BranchStatus = "closed"
)

type Branch struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	Status      BranchStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BranchList struct {
	Items  []Branch
	Limit  int
	Offset int
	Total  int
}
