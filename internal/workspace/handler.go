package workspace

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pp-sem7-team/backend/internal/httpx"
	"github.com/pp-sem7-team/backend/internal/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Create godoc
// @Summary Create workspace
// @Description Creates a workspace for the authenticated user.
// @Tags workspaces
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateWorkspaceRequest true "Workspace data"
// @Success 201 {object} WorkspaceResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces [post]
func (h *Handler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	var req CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid request body")
		return
	}

	workspace, err := h.service.Create(c.Request.Context(), userID, req.Name)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNameExists) {
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Workspace name already exists")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to create workspace")
		return
	}

	c.JSON(http.StatusCreated, toWorkspaceResponse(workspace))
}

// List godoc
// @Summary List workspaces
// @Description Returns the workspaces of the authenticated user.
// @Tags workspaces
// @Security BearerAuth
// @Produce json
// @Param offset query int false "Number of items to skip" default(0) minimum(0)
// @Param limit query int false "Maximum number of items" default(10) minimum(1) maximum(100)
// @Success 200 {object} WorkspaceListResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces [get]
func (h *Handler) List(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 || limit > 100 {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid limit")
		return
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid offset")
		return
	}

	result, err := h.service.List(c.Request.Context(), userID, int32(limit), int32(offset))
	if err != nil {
		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to list workspaces")
		return
	}

	items := make([]WorkspaceResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toWorkspaceResponse(item))
	}

	c.JSON(http.StatusOK, WorkspaceListResponse{
		Items:  items,
		Limit:  result.Limit,
		Offset: result.Offset,
		Total:  result.Total,
	})
}

// Get godoc
// @Summary Get workspace
// @Description Returns a workspace owned by the authenticated user.
// @Tags workspaces
// @Security BearerAuth
// @Produce json
// @Param workspace_id path string true "Workspace ID"
// @Success 200 {object} WorkspaceResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces/{workspace_id} [get]
func (h *Handler) Get(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspace_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid workspace ID")
		return
	}

	workspace, err := h.service.Get(c.Request.Context(), userID, workspaceID)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Workspace not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to get workspace")
		return
	}

	c.JSON(http.StatusOK, toWorkspaceResponse(workspace))
}

// Rename godoc
// @Summary Rename workspace
// @Description Renames a workspace owned by the authenticated user.
// @Tags workspaces
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace_id path string true "Workspace ID"
// @Param request body RenameWorkspaceRequest true "New workspace name"
// @Success 200 {object} WorkspaceResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces/{workspace_id} [patch]
func (h *Handler) Rename(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspace_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid workspace ID")
		return
	}

	var req RenameWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid request body")
		return
	}

	workspace, err := h.service.Rename(c.Request.Context(), userID, workspaceID, req.Name)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Workspace not found")
			return
		}
		if errors.Is(err, ErrWorkspaceNameExists) {
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Workspace name already exists")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to rename workspace")
		return
	}

	c.JSON(http.StatusOK, toWorkspaceResponse(workspace))
}

// Delete godoc
// @Summary Delete workspace
// @Description Deletes a workspace and its related data.
// @Tags workspaces
// @Security BearerAuth
// @Param workspace_id path string true "Workspace ID"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces/{workspace_id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	workspaceID, err := uuid.Parse(c.Param("workspace_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid workspace ID")
		return
	}

	err = h.service.Delete(c.Request.Context(), userID, workspaceID)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Workspace not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to delete workspace")
		return
	}

	c.Status(http.StatusNoContent)
}
