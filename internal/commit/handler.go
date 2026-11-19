package commit

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

// RegenerateCommit godoc
// @Summary Regenerate a commit message
// @Tags commits
// @Security BearerAuth
// @Produce json
// @Param change_id path string true "Change ID"
// @Success 202 {object} CommitResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /changes/{change_id}/commit [post]
func (h *Handler) RegenerateCommit(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	changeID, err := uuid.Parse(c.Param("change_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid change ID")
		return
	}

	commit, err := h.service.RegenerateCommit(c.Request.Context(), userID, changeID)
	if err != nil {
		switch {
		case errors.Is(err, ErrCommitNotFound):
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Commit not found")
		case errors.Is(err, ErrAnalysisNotDone):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Analysis is not done")
		case errors.Is(err, ErrCommitPending):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Commit generation is pending")
		default:
			httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to regenerate commit")
		}
		return
	}

	c.JSON(http.StatusAccepted, ToCommitResponse(commit))
}

// EditCommit godoc
// @Summary Edit a commit message
// @Tags commits
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param change_id path string true "Change ID"
// @Param request body EditCommitRequest true "Commit message"
// @Success 200 {object} CommitResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 413 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /changes/{change_id}/commit [patch]
func (h *Handler) EditCommit(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(c, http.StatusUnauthorized, httpx.ErrorCodeUnauthorized, "Unauthorized")
		return
	}

	changeID, err := uuid.Parse(c.Param("change_id"))
	if err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid change ID")
		return
	}

	var req EditCommitRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if !req.Type.IsValid() || strings.TrimSpace(req.Message) == "" {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid commit type or empty message")
		return
	}

	edited, err := h.service.EditCommit(c.Request.Context(), userID, changeID, req.Type, req.Scope, req.Message)
	if err != nil {
		switch {
		case errors.Is(err, ErrCommitNotFound):
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Commit not found")
		case errors.Is(err, ErrCommitNotDone):
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Commit generation is not done")
		default:
			httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to edit commit")
		}
		return
	}

	c.JSON(http.StatusOK, ToCommitResponse(edited))
}
