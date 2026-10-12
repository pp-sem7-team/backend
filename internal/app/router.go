package app

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/pp-sem7-team/backend/docs"

	"github.com/pp-sem7-team/backend/internal/auth"
	"github.com/pp-sem7-team/backend/internal/branch"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/health"
	"github.com/pp-sem7-team/backend/internal/middleware"
	"github.com/pp-sem7-team/backend/internal/user"
	"github.com/pp-sem7-team/backend/internal/workspace"
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

	workspaceService := workspace.NewService(querier, log.With("module", "workspace"))
	workspaceHandler := workspace.NewHandler(workspaceService)

	branchService := branch.NewService(querier, log.With("module", "branch"))
	branchHandler := branch.NewHandler(branchService)

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

	workspaces := router.Group("/workspaces")
	workspaces.Use(middleware.Auth(tokenManager))
	{
		workspaces.POST("", workspaceHandler.Create)
		workspaces.GET("", workspaceHandler.List)
		workspaces.GET("/:workspace_id", workspaceHandler.Get)
		workspaces.PATCH("/:workspace_id", workspaceHandler.Rename)
		workspaces.DELETE("/:workspace_id", workspaceHandler.Delete)
	}

	branches := router.Group("")
	branches.Use(middleware.Auth(tokenManager))
	{
		branches.POST("/workspaces/:workspace_id/branches", branchHandler.Create)
		branches.GET("/workspaces/:workspace_id/branches", branchHandler.List)
		branches.GET("/branches/:branch_id", branchHandler.Get)
		branches.PATCH("/branches/:branch_id", branchHandler.Update)
		branches.DELETE("/branches/:branch_id", branchHandler.Delete)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
