package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pp-sem7-team/backend/internal/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register godoc
// @Summary Register user
// @Description Registers a new user and returns an access and refresh token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CredentialsRequest true "Registration credentials"
// @Success 201 {object} TokenPair
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(
			c,
			http.StatusBadRequest,
			httpx.ErrorCodeInvalidRequest,
			"Invalid request body",
		)
		return
	}

	tokens, err := h.service.Register(
		c.Request.Context(),
		req.Email,
		req.Password,
	)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			httpx.WriteError(
				c,
				http.StatusConflict,
				httpx.ErrorCodeConflict,
				"Email already exists",
			)
			return
		}

		httpx.WriteError(
			c,
			http.StatusInternalServerError,
			httpx.ErrorCodeInternalError,
			"Failed to register user",
		)
		return
	}

	c.JSON(http.StatusCreated, tokens)
}

// Login godoc
// @Summary Login user
// @Description Authenticates a user and returns an access and refresh token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CredentialsRequest true "Login credentials"
// @Success 200 {object} TokenPair
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(
			c,
			http.StatusBadRequest,
			httpx.ErrorCodeInvalidRequest,
			"Invalid request body",
		)
		return
	}

	tokens, err := h.service.Login(
		c.Request.Context(),
		req.Email,
		req.Password,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.WriteError(
				c,
				http.StatusUnauthorized,
				httpx.ErrorCodeUnauthorized,
				"Invalid email or password",
			)
			return
		}

		httpx.WriteError(
			c,
			http.StatusInternalServerError,
			httpx.ErrorCodeInternalError,
			"Failed to authenticate user",
		)
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// Refresh godoc
// @Summary Refresh token pair
// @Description Revokes the provided refresh token and returns a new token pair.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} TokenPair
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(
			c,
			http.StatusBadRequest,
			httpx.ErrorCodeInvalidRequest,
			"Invalid request body",
		)
		return
	}

	tokens, err := h.service.Refresh(
		c.Request.Context(),
		req.RefreshToken,
	)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			httpx.WriteError(
				c,
				http.StatusUnauthorized,
				httpx.ErrorCodeUnauthorized,
				"Invalid or expired refresh token",
			)
			return
		}

		httpx.WriteError(
			c,
			http.StatusInternalServerError,
			httpx.ErrorCodeInternalError,
			"Failed to refresh tokens",
		)
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// Logout godoc
// @Summary Logout user
// @Description Revokes the provided refresh token.
// @Tags auth
// @Accept json
// @Param request body RefreshTokenRequest true "Refresh token"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(
			c,
			http.StatusBadRequest,
			httpx.ErrorCodeInvalidRequest,
			"Invalid request body",
		)
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.WriteError(
			c,
			http.StatusInternalServerError,
			httpx.ErrorCodeInternalError,
			"Failed to logout",
		)
		return
	}

	c.Status(http.StatusNoContent)
}
