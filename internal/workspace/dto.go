package workspace

import "time"

type WorkspaceResponse struct {
	ID        string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string    `json:"name" example:"My project"`
	CreatedAt time.Time `json:"created_at" example:"2026-01-01T12:00:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2026-01-01T12:00:00Z"`
}

type CreateWorkspaceRequest struct {
	Name string `json:"name" binding:"required" example:"My project"`
}

type RenameWorkspaceRequest struct {
	Name string `json:"name" binding:"required" example:"New project name"`
}

type WorkspaceListResponse struct {
	Items  []WorkspaceResponse `json:"items"`
	Limit  int32               `json:"limit" example:"10"`
	Offset int32               `json:"offset" example:"0"`
	Total  int64               `json:"total" example:"25"`
}

func toWorkspaceResponse(workspace Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:        workspace.ID.String(),
		Name:      workspace.Name,
		CreatedAt: workspace.CreatedAt,
		UpdatedAt: workspace.UpdatedAt,
	}
}
