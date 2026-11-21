package pullrequest

import (
	"errors"
	"net/http"
	"strings"

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

// Generate godoc
// @Summary Generate a pull request title and description
// @Tags pull-requests
// @Security BearerAuth
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Success 202 {object} PullRequestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id}/pull-request [post]
func (h *Handler) Generate(c *gin.Context) {
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

	result, err := h.service.Generate(c.Request.Context(), userID, branchID)
	if err != nil {
		switch {
		case errors.Is(err, ErrBranchNotFound):
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
		case errors.Is(err, ErrNoChanges):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Branch has no changes")
		case errors.Is(err, ErrBranchAnalysesNotDone):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "All changes in the branch must be analyzed successfully")
		case errors.Is(err, ErrPullRequestPending):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Pull request generation is pending")
		default:
			httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to generate pull request")
		}
		return
	}
	c.JSON(http.StatusAccepted, toPullRequestResponse(result))
}

// Get godoc
// @Summary Get a branch pull request
// @Tags pull-requests
// @Security BearerAuth
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Success 200 {object} PullRequestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id}/pull-request [get]
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

	result, err := h.service.Get(c.Request.Context(), userID, branchID)
	if err != nil {
		switch {
		case errors.Is(err, ErrPullRequestNotFound):
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Pull request not found")
		default:
			httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to get pull request")
		}
		return
	}
	c.JSON(http.StatusOK, toPullRequestResponse(result))
}

// Edit godoc
// @Summary Edit a generated pull request
// @Tags pull-requests
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Param request body EditPullRequestRequest true "Pull request title and description"
// @Success 200 {object} PullRequestResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 413 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id}/pull-request [patch]
func (h *Handler) Edit(c *gin.Context) {
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

	var req EditPullRequestRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Description) == "" {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Title and description are required")
		return
	}
	result, err := h.service.Edit(c.Request.Context(), userID, branchID, req.Title, req.Description)
	if err != nil {
		switch {
		case errors.Is(err, ErrBranchNotFound):
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
		case errors.Is(err, ErrPullRequestNotFound):
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Pull request not found")
		case errors.Is(err, ErrPullRequestNotDone):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Pull request generation is not done")
		default:
			httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to edit pull request")
		}
		return
	}
	c.JSON(http.StatusOK, toPullRequestResponse(result))
}
