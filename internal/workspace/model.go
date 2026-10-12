package workspace

import (
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WorkspaceList struct {
	Items  []Workspace
	Limit  int32
	Offset int32
	Total  int64
}
