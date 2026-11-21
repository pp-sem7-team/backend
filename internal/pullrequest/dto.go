package pullrequest

import (
	"time"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type PullRequestResponse struct {
	ID             string            `json:"id" example:"550e8400-e29b-41d4-a716-446655440004"`
	BranchID       string            `json:"branch_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Title          *string           `json:"title" extensions:"x-nullable" example:"Implement user service"`
	Description    *string           `json:"description" extensions:"x-nullable" example:"Реализован модуль пользователей."`
	IsEditedByUser bool              `json:"is_edited_by_user" example:"false"`
	Status         generation.Status `json:"status" example:"pending"`
	CreatedAt      time.Time         `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt      time.Time         `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

type EditPullRequestRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
}

func toPullRequestResponse(pr PullRequest) PullRequestResponse {
	return PullRequestResponse{
		ID:             pr.ID.String(),
		BranchID:       pr.BranchID.String(),
		Title:          pr.Title,
		Description:    pr.Description,
		IsEditedByUser: pr.IsEditedByUser,
		Status:         pr.Status,
		CreatedAt:      pr.CreatedAt,
		UpdatedAt:      pr.UpdatedAt,
	}
}
