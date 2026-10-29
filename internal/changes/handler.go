package changes

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
// @Summary Create change
// @Description Creates a change in a branch, starts asynchronous analysis and returns 202 Accepted.
// @Tags changes
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Param request body CreateChangeRequest true "Change data"
// @Success 202 {object} ChangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id}/changes [post]
func (h *Handler) Create(c *gin.Context) {
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

	var req CreateChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Invalid request body")
		return
	}

	change, err := h.service.Create(c.Request.Context(), userID, branchID, req.RawDiff)
	if err != nil {
		if errors.Is(err, ErrEmptyRawDiff) {
			httpx.WriteError(c, http.StatusBadRequest, httpx.ErrorCodeInvalidRequest, "Raw diff is required")
			return
		}
		if errors.Is(err, ErrBranchNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
			return
		}
		if errors.Is(err, ErrBranchClosed) {
			httpx.WriteError(c, http.StatusConflict, httpx.ErrorCodeConflict, "Branch is closed")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to create change")
		return
	}

	c.JSON(http.StatusAccepted, toChangeResponse(change))
}

// List godoc
// @Summary List changes
// @Description Returns changes of a branch ordered by sequence number.
// @Tags changes
// @Security BearerAuth
// @Produce json
// @Param branch_id path string true "Branch ID"
// @Param offset query int false "Number of items to skip" default(0) minimum(0)
// @Param limit query int false "Maximum number of items" default(10) minimum(1) maximum(100)
// @Success 200 {object} ChangeListResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /branches/{branch_id}/changes [get]
func (h *Handler) List(c *gin.Context) {
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

	result, err := h.service.List(c.Request.Context(), userID, branchID, int32(limit), int32(offset))
	if err != nil {
		if errors.Is(err, ErrBranchNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Branch not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to list changes")
		return
	}

	c.JSON(http.StatusOK, toChangeListResponse(result))
}

// Get godoc
// @Summary Get change
// @Description Returns a change with its analysis result.
// @Tags changes
// @Security BearerAuth
// @Produce json
// @Param change_id path string true "Change ID"
// @Success 200 {object} ChangeResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /changes/{change_id} [get]
func (h *Handler) Get(c *gin.Context) {
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

	change, err := h.service.Get(c.Request.Context(), userID, changeID)
	if err != nil {
		if errors.Is(err, ErrChangeNotFound) {
			httpx.WriteError(c, http.StatusNotFound, httpx.ErrorCodeNotFound, "Change not found")
			return
		}

		httpx.WriteError(c, http.StatusInternalServerError, httpx.ErrorCodeInternalError, "Failed to get change")
		return
	}

	c.JSON(http.StatusOK, toChangeResponse(change))
}
