package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/pp-sem7-team/backend/internal/config"
	"github.com/pp-sem7-team/backend/internal/db"
	"github.com/pp-sem7-team/backend/internal/logger"
)

func Run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg := config.Load()

	log := logger.New(cfg.Logging.Level)

	ctx := context.Background()

	database, err := db.New(ctx, cfg.Postgres, log)
	if err != nil {
		return err
	}
	defer database.Close()

	router := NewRouter(database, log)

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
		if err == http.ErrServerClosed {
			return nil
		}

		log.Error("server failed", "error", err)
		return err

	case sig := <-signalCh:
		log.Info("shutdown signal received", "signal", sig.String())

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
