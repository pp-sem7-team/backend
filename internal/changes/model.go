package changes

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AnalysisStatus string

const (
	AnalysisStatusPending AnalysisStatus = "pending"
	AnalysisStatusDone    AnalysisStatus = "done"
	AnalysisStatusFailed  AnalysisStatus = "failed"
)

type Change struct {
	ID             uuid.UUID
	BranchID       uuid.UUID
	SequenceNumber int
	Analysis       AnalysisResult
	CreatedAt      time.Time
}

type AnalysisResult struct {
	ID                uuid.UUID
	StructuredChanges json.RawMessage
	Status            AnalysisStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ChangeList struct {
	Items  []Change
	Limit  int
	Offset int
	Total  int
}
