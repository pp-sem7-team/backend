package branch

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
// @Summary Create branch
// @Description Creates a branch in a workspace owned by the authenticated user.
// @Tags branches
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace_id path string true "Workspace ID"
// @Param request body CreateBranchRequest true "Branch data"
// @Success 201 {object} BranchResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces/{workspace_id}/branches [post]
func (h *Handler) Create(c *gin.Context) {
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

	var req CreateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid request body")
		return
	}

	branch, err := h.service.Create(c.Request.Context(), userID, workspaceID, req.Name)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Workspace not found")
			return
		}
		if errors.Is(err, ErrBranchNameExists) {
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Branch name already exists")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to create branch")
		return
	}

	c.JSON(http.StatusCreated, toBranchResponse(branch))
}

// List godoc
// @Summary List branches
// @Description Returns branches in a workspace owned by the authenticated user.
// @Tags branches
// @Security BearerAuth
// @Produce json
// @Param workspace_id path string true "Workspace ID"
// @Param offset query int false "Number of items to skip" default(0) minimum(0)
// @Param limit query int false "Maximum number of items" default(10) minimum(1) maximum(100)
// @Param status query string false "Branch status" Enums(active, closed)
// @Success 200 {object} BranchListResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /workspaces/{workspace_id}/branches [get]
func (h *Handler) List(c *gin.Context) {
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

	var status *BranchStatus
	if value := c.Query("status"); value != "" {
		parsed := BranchStatus(value)
		if parsed != BranchStatusActive && parsed != BranchStatusClosed {
			httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid branch status")
			return
		}
		status = &parsed
	}

	result, err := h.service.List(c.Request.Context(), userID, workspaceID, int32(limit), int32(offset), status)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Workspace not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to list branches")
		return
	}

	c.JSON(http.StatusOK, toBranchListResponse(result))
}

// Get godoc
// @Summary Get branch
// @Description Returns a branch owned by the authenticated user.
// @Tags branches
// @Security BearerAuth
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Success 200 {object} BranchResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id} [get]
func (h *Handler) Get(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid branch ID")
		return
	}

	branch, err := h.service.Get(c.Request.Context(), userID, branchID)
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to get branch")
		return
	}

	c.JSON(http.StatusOK, toBranchResponse(branch))
}

// Update godoc
// @Summary Update branch
// @Description Updates a branch owned by the authenticated user.
// @Tags branches
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Param request body UpdateBranchRequest true "Branch fields to update"
// @Success 200 {object} BranchResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id} [patch]
func (h *Handler) Update(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid branch ID")
		return
	}

	var req UpdateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid request body")
		return
	}

	branch, err := h.service.Update(c.Request.Context(), userID, branchID, req.Name, req.Status)
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
			return
		}
		if errors.Is(err, ErrBranchNameExists) {
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Branch name already exists")
			return
		}
		if errors.Is(err, ErrEmptyUpdate) {
			httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "No fields to update")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to update branch")
		return
	}

	c.JSON(http.StatusOK, toBranchResponse(branch))
}

// Delete godoc
// @Summary Delete branch
// @Description Deletes a branch and its related data.
// @Tags branches
// @Security BearerAuth
// @Param branch_id path string true "Branch ID"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	branchID, err := uuid.Parse(c.Param("branch_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid branch ID")
		return
	}

	err = h.service.Delete(c.Request.Context(), userID, branchID)
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to delete branch")
		return
	}

	c.Status(http.StatusNoContent)
}
