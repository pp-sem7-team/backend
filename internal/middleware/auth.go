package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pp-sem7-team/backend/internal/httpx"
)

const userIDKey = "userID"

type AccessTokenValidator interface {
	ValidateAccessToken(rawToken string) (uuid.UUID, error)
}

func Auth(tokenValidator AccessTokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.WriteError(
				c,
				http.StatusUnauthorized,
				httpx.ErrorCodeUnauthorized,
				"Invalid or missing access token",
			)
			c.Abort()
			return
		}

		userID, err := tokenValidator.ValidateAccessToken(parts[1])
		if err != nil {
			httpx.WriteError(
				c,
				http.StatusUnauthorized,
				httpx.ErrorCodeUnauthorized,
				"Invalid or missing access token",
			)
			c.Abort()
			return
		}

		c.Set(userIDKey, userID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(userIDKey)
	if !exists {
		return uuid.Nil, false
	}

	userID, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, false
	}

	return userID, true
}
