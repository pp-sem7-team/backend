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
	"github.com/pp-sem7-team/backend/internal/broker"
	"github.com/pp-sem7-team/backend/internal/changes"
	"github.com/pp-sem7-team/backend/internal/cleanup"
	"github.com/pp-sem7-team/backend/internal/config"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/logger"
	"github.com/pp-sem7-team/backend/internal/outbox"
)

const analysisResultPrefetch = 10

func Run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg := config.Load()

	log := logger.New(cfg.Logging.Level)

	ctx := context.Background()

	database, err := db.NewDB(ctx, cfg.Postgres, log)
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

	changesService := changes.NewService(database, querier, log.With("module", "changes"))
	analysisConsumer := changes.NewAnalysisResultConsumer(changesService, log.With("module", "analysis"))
	outboxWorker := outbox.NewWorker(
		querier,
		brokerClient,
		cfg.Outbox.Retention,
		log.With("module", "outbox"),
	)

	workers.Add(4)

	go func() {
		defer workers.Done()
		outboxWorker.Run(appCtx)
	}()

	go func() {
		defer workers.Done()
		changesService.RunTimeoutWorker(appCtx, cfg.Generation.Timeout)
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
		cleanup.RefreshTokens(appCtx, querier, log)
	}()

	router := NewRouter(database, querier, tokenManager, changesService, log)

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
