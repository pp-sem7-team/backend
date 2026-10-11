package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/httpx"
)

type HealthHandler struct {
	db *db.DB
}

func NewHealthHandler(database *db.DB) *HealthHandler {
	return &HealthHandler{
		db: database,
	}
}

// Health godoc
// @Summary Health check
// @Description Checks that the server is running.
// @Tags health
// @Success 200
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	c.Status(http.StatusOK)
}

// HealthDB godoc
// @Summary Database health check
// @Description Checks that the server can connect to PostgreSQL.
// @Tags health
// @Success 200
// @Failure 503 {object} httpx.ErrorResponse
// @Router /health/db [get]
func (h *HealthHandler) HealthDB(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		httpx.WriteError(
			c,
			http.StatusServiceUnavailable,
			httpx.ErrorCodeUnavailable,
			"database is unavailable",
		)
		return
	}

	c.Status(http.StatusOK)
}
