package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/pp-sem7-team/backend/internal/auth"
	"github.com/pp-sem7-team/backend/internal/branch"
	"github.com/pp-sem7-team/backend/internal/broker"
	"github.com/pp-sem7-team/backend/internal/change"
	"github.com/pp-sem7-team/backend/internal/cleanup"
	"github.com/pp-sem7-team/backend/internal/commit"
	"github.com/pp-sem7-team/backend/internal/config"
	"github.com/pp-sem7-team/backend/internal/logger"
	"github.com/pp-sem7-team/backend/internal/outbox"
	"github.com/pp-sem7-team/backend/internal/postgres"
	"github.com/pp-sem7-team/backend/internal/postgres/db"
	"github.com/pp-sem7-team/backend/internal/pullrequest"
	"github.com/pp-sem7-team/backend/internal/user"
	"github.com/pp-sem7-team/backend/internal/workspace"
)

const analysisResultPrefetch = 10
const commitResultPrefetch = 10

func Run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg := config.Load()

	log := logger.New(cfg.Logging.Level)

	ctx := context.Background()

	database, err := postgres.NewDB(ctx, cfg.Postgres, log)
	if err != nil {
		return err
	}
	defer database.Close()

	brokerClient, err := broker.NewClient(cfg.RabbitMQ, log)
	if err != nil {
		return err
	}
	defer brokerClient.Close()

	querier := db.New(database.Pool)

	tokenManager, err := auth.NewJWTTokenManager(cfg.Auth.JWTSecret)
	if err != nil {
		return err
	}

	appCtx, stopApp := context.WithCancel(context.Background())
	defer stopApp()

	var workers sync.WaitGroup

	authService := auth.NewService(querier, tokenManager, log.With("module", "auth"))
	userService := user.NewService(querier, log.With("module", "user"))
	workspaceService := workspace.NewService(querier, log.With("module", "workspace"))
	branchService := branch.NewService(querier, log.With("module", "branch"))
	changeService := change.NewService(database, querier, log.With("module", "change"))
	commitService := commit.NewService(database, querier, log.With("module", "commit"))
	pullRequestService := pullrequest.NewService(database, querier, log.With("module", "pullrequest"))

	analysisConsumer := change.NewAnalysisResultConsumer(changeService, log.With("module", "analysis"))
	commitConsumer := commit.NewCommitResultConsumer(commitService, log.With("module", "commit"))
	outboxWorker := outbox.NewWorker(
		querier,
		brokerClient,
		cfg.Outbox.Retention,
		log.With("module", "outbox"),
	)

	workers.Add(6)

	go func() {
		defer workers.Done()
		outboxWorker.Run(appCtx)
	}()

	go func() {
		defer workers.Done()
		changeService.RunTimeoutWorker(appCtx, cfg.Generation.Timeout)
	}()

	go func() {
		defer workers.Done()
		commitService.RunTimeoutWorker(appCtx, cfg.Generation.Timeout)
	}()

	go func() {
		defer workers.Done()

		err := brokerClient.Consume(
			appCtx,
			broker.AnalysisResultsQueue,
			analysisResultPrefetch,
			analysisConsumer.Handle,
		)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("analysis result consumer stopped", "error", err)
		}
	}()

	go func() {
		defer workers.Done()

		err := brokerClient.Consume(appCtx,
			broker.CommitResultsQueue,
			commitResultPrefetch,
			commitConsumer.Handle,
		)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error("commit result consumer stopped", "error", err)
		}
	}()

	go func() {
		defer workers.Done()
		cleanup.RefreshTokens(appCtx, querier, log)
	}()

	router := NewRouter(
		database,
		tokenManager,
		authService,
		userService,
		workspaceService,
		branchService,
		changeService,
		commitService,
		pullRequestService,
		log,
	)

	server := &http.Server{
		Addr:    cfg.Server.Host + ":" + cfg.Server.Port,
		Handler: router,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Info("server starting", "addr", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	select {
	case err := <-serverErrors:
		stopApp()
		workers.Wait()

		if err == http.ErrServerClosed {
			return nil
		}

		log.Error("server failed", "error", err)
		return err

	case sig := <-signalCh:
		log.Info("shutdown signal received", "signal", sig.String())

		stopApp()
		workers.Wait()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Error("server shutdown failed", "error", err)
			return err
		}

		log.Info("server stopped")
		return nil
	}
}
