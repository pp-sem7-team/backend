package changes

import (
	"encoding/json"
	"time"
)

type CreateChangeRequest struct {
	RawDiff string `json:"raw_diff" binding:"required" example:"diff --git a/main.go b/main.go"`
}

type ChangeResponse struct {
	ID             string                 `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	BranchID       string                 `json:"branch_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	SequenceNumber int32                  `json:"sequence_number" example:"1"`
	Analysis       AnalysisResultResponse `json:"analysis"`
	// Commit is nil until the commit flow is implemented.
	Commit    *CommitResponse `json:"commit"`
	CreatedAt time.Time       `json:"created_at" example:"2026-01-01T12:00:00Z"`
}

type AnalysisResultResponse struct {
	ID                string          `json:"id" example:"550e8400-e29b-41d4-a716-446655440002"`
	StructuredChanges json.RawMessage `json:"structured_changes" swaggertype:"object"`
	Status            AnalysisStatus  `json:"status" example:"pending"`
	CreatedAt         time.Time       `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt         time.Time       `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

type CommitResponse struct {
	ID             string         `json:"id" example:"550e8400-e29b-41d4-a716-446655440003"`
	Type           *string        `json:"type" example:"feat"`
	Scope          *string        `json:"scope" example:"api"`
	Message        *string        `json:"message" example:"add changes endpoint"`
	IsEditedByUser bool           `json:"is_edited_by_user" example:"false"`
	Status         AnalysisStatus `json:"status" example:"pending"`
	CreatedAt      time.Time      `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt      time.Time      `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

type ChangeListResponse struct {
	Items  []ChangeResponse `json:"items"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
	Total  int64            `json:"total"`
}

func toChangeResponse(change Change) ChangeResponse {
	return ChangeResponse{
		ID:             change.ID.String(),
		BranchID:       change.BranchID.String(),
		SequenceNumber: change.SequenceNumber,
		Analysis: AnalysisResultResponse{
			ID:                change.Analysis.ID.String(),
			StructuredChanges: change.Analysis.StructuredChanges,
			Status:            change.Analysis.Status,
			CreatedAt:         change.Analysis.CreatedAt,
			UpdatedAt:         change.Analysis.UpdatedAt,
		},
		Commit:    nil,
		CreatedAt: change.CreatedAt,
	}
}

func toChangeListResponse(list ChangeList) ChangeListResponse {
	items := make([]ChangeResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, toChangeResponse(item))
	}

	return ChangeListResponse{
		Items:  items,
		Limit:  list.Limit,
		Offset: list.Offset,
		Total:  list.Total,
	}
}
