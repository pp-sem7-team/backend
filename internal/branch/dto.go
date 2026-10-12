package branch

import "time"

type BranchResponse struct {
	ID          string       `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	WorkspaceID string       `json:"workspace_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Name        string       `json:"name" example:"feature/user-search"`
	Status      BranchStatus `json:"status" example:"active"`
	CreatedAt   time.Time    `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt   time.Time    `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

type CreateBranchRequest struct {
	Name string `json:"name" binding:"required" example:"feature/user-search"`
}

type UpdateBranchRequest struct {
	Name   *string       `json:"name" example:"feature/user-search"`
	Status *BranchStatus `json:"status" example:"closed"`
}
type BranchListResponse struct {
	Items  []BranchResponse `json:"items"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
	Total  int64            `json:"total"`
}

func toBranchResponse(branch Branch) BranchResponse {
	return BranchResponse{
		ID:          branch.ID.String(),
		WorkspaceID: branch.WorkspaceID.String(),
		Name:        branch.Name,
		Status:      branch.Status,
		CreatedAt:   branch.CreatedAt,
		UpdatedAt:   branch.UpdatedAt,
	}
}

func toBranchListResponse(list BranchList) BranchListResponse {
	items := make([]BranchResponse, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, toBranchResponse(item))
	}

	return BranchListResponse{
		Items:  items,
		Limit:  list.Limit,
		Offset: list.Offset,
		Total:  list.Total,
	}
}
