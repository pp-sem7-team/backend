package commit

import (
	"time"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type CommitResponse struct {
	ID             string            `json:"id" example:"550e8400-e29b-41d4-a716-446655440003"`
	Type           *string           `json:"type" example:"feat"`
	Scope          *string           `json:"scope" example:"api"`
	Message        *string           `json:"message" example:"add changes endpoint"`
	IsEditedByUser bool              `json:"is_edited_by_user" example:"false"`
	Status         generation.Status `json:"status" example:"pending"`
	CreatedAt      time.Time         `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt      time.Time         `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

func ToCommitResponse(commit *Commit) *CommitResponse {
	if commit == nil {
		return nil
	}

	return &CommitResponse{
		ID:             commit.ID.String(),
		Type:           commit.Type,
		Scope:          commit.Scope,
		Message:        commit.Message,
		IsEditedByUser: commit.IsEditedByUser,
		Status:         commit.Status,
		CreatedAt:      commit.CreatedAt,
		UpdatedAt:      commit.UpdatedAt,
	}
}
