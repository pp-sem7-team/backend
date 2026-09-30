package app

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/pp-sem7-team/backend/docs"

	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/handler"
	"github.com/pp-sem7-team/backend/internal/middleware"
)

func NewRouter(
	database *db.DB,
	log *slog.Logger,
) http.Handler {
	router := gin.New()

	router.Use(middleware.Logging(log))

	healthHandler := handler.NewHealthHandler(database)

	router.GET("/health", healthHandler.Health)
	router.GET("/health/db", healthHandler.HealthDB)

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
