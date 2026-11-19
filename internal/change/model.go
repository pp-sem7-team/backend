package change

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/commit"
	"github.com/pp-sem7-team/backend/internal/generation"
)

type Change struct {
	ID             uuid.UUID
	BranchID       uuid.UUID
	SequenceNumber int
	Analysis       AnalysisResult
	Commit         *commit.Commit
	CreatedAt      time.Time
}

type AnalysisResult struct {
	ID                uuid.UUID
	StructuredChanges json.RawMessage
	Status            generation.Status
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ChangeList struct {
	Items  []Change
	Limit  int
	Offset int
	Total  int
}
