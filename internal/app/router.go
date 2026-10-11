package app

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/pp-sem7-team/backend/docs"

	"github.com/pp-sem7-team/backend/internal/auth"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/health"
	"github.com/pp-sem7-team/backend/internal/middleware"
	"github.com/pp-sem7-team/backend/internal/user"
)

func NewRouter(
	database *db.DB,
	querier *db.Queries,
	tokenManager *auth.JWTTokenManager,
	log *slog.Logger,
) http.Handler {
	router := gin.New()

	router.Use(middleware.Logging(log))

	healthHandler := health.NewHealthHandler(database)

	authService := auth.NewService(querier, tokenManager, log.With("module", "auth"))
	authHandler := auth.NewHandler(authService)

	userService := user.NewService(querier, log.With("module", "user"))
	userHandler := user.NewHandler(userService)

	router.GET("/health", healthHandler.Health)
	router.GET("/health/db", healthHandler.HealthDB)

	authRoutes := router.Group("/auth")
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)
		authRoutes.POST("/refresh", authHandler.Refresh)
		authRoutes.POST("/logout", authHandler.Logout)
	}

	users := router.Group("/users")
	users.Use(middleware.Auth(tokenManager))
	{
		users.GET("/me", userHandler.GetMe)
		users.PATCH("/me/password", userHandler.ChangePassword)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
