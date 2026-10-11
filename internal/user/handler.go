package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pp-sem7-team/backend/internal/httpx"
	"github.com/pp-sem7-team/backend/internal/middleware"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetMe godoc
// @Summary Get current user
// @Description Returns the profile of the authenticated user.
// @Tags users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} UserResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /users/me [get]
func (h *Handler) GetMe(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(
			c,
			http.StatusUnauthorized,
			httpx.ErrorCodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	user, err := h.service.GetMe(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpx.WriteError(
				c,
				http.StatusNotFound,
				httpx.ErrorCodeNotFound,
				"User not found",
			)
			return
		}

		httpx.WriteError(
			c,
			http.StatusInternalServerError,
			httpx.ErrorCodeInternalError,
			"Failed to get user profile",
		)
		return
	}

	c.JSON(http.StatusOK, UserResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	})
}

// ChangePassword godoc
// @Summary Change current user's password
// @Description Changes the password of the authenticated user.
// @Tags users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body UpdatePasswordRequest true "Current and new passwords"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /users/me/password [patch]
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		httpx.WriteError(
			c,
			http.StatusUnauthorized,
			httpx.ErrorCodeUnauthorized,
			"Unauthorized",
		)
		return
	}

	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(
			c,
			http.StatusBadRequest,
			httpx.ErrorCodeInvalidRequest,
			"Invalid request body",
		)
		return
	}

	err := h.service.ChangePassword(
		c.Request.Context(),
		userID,
		req.CurrentPassword,
		req.NewPassword,
	)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpx.WriteError(
				c,
				http.StatusNotFound,
				httpx.ErrorCodeNotFound,
				"User not found",
			)
			return
		}
		if errors.Is(err, ErrInvalidPassword) {
			httpx.WriteError(
				c,
				http.StatusBadRequest,
				httpx.ErrorCodeInvalidRequest,
				"Current password is incorrect",
			)
			return
		}

		httpx.WriteError(
			c,
			http.StatusInternalServerError,
			httpx.ErrorCodeInternalError,
			"Failed to change password",
		)
		return
	}

	c.Status(http.StatusNoContent)
}
