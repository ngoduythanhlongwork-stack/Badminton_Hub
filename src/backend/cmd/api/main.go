package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"badmintonhub/internal/config"
	"badmintonhub/internal/httpapi"
	"badmintonhub/internal/platform/connections"
	"badmintonhub/internal/platform/outbox"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) (runErr error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	clients, err := connections.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer clients.Close()
	pingCtx, cancelPing := context.WithTimeout(ctx, cfg.DependencyTimeout)
	if err := clients.PingRedis(pingCtx); err != nil {
		logger.Warn("Redis unavailable; continuing without cache")
	} else {
		logger.Info("Redis connected")
	}
	cancelPing()
	logger.Info("PostgreSQL connected")
	r1, err := composeR1(clients.Postgres, cfg, logger)
	if err != nil {
		return fmt.Errorf("compose R1 modules: %w", err)
	}
	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandlerWithOptions(httpapi.Options{
			Readiness: httpapi.Readiness{Postgres: clients.Postgres.Ping, Redis: clients.PingRedis, Timeout: cfg.DependencyTimeout},
			Logger:    logger,
			Register:  r1.routes.Register,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	worker := startWorker(ctx, outbox.Worker{Pool: clients.Postgres, Handlers: r1.handlers, Logger: logger})
	defer func() {
		if err := worker.Stop(10 * time.Second); err != nil {
			if runErr == nil {
				runErr = err
			} else if !errors.Is(runErr, err) {
				logger.Error("Outbox worker did not stop cleanly", "error", err)
			}
		}
	}()
	logger.Info("API starting", "address", cfg.HTTPAddr)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-worker.done:
		if ctx.Err() != nil {
			return shutdownServer(server, result, logger)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := server.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			_ = server.Close()
			logger.Error("HTTP shutdown after worker termination failed", "error", shutdownErr)
		}
		<-result
		if worker.err != nil {
			return fmt.Errorf("outbox worker stopped unexpectedly: %w", worker.err)
		}
		return errors.New("outbox worker stopped unexpectedly")
	case <-ctx.Done():
		return shutdownServer(server, result, logger)
	}
}

type managedWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func startWorker(parent context.Context, worker outbox.Worker) *managedWorker {
	ctx, cancel := context.WithCancel(parent)
	managed := &managedWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		managed.err = worker.Run(ctx)
		close(managed.done)
	}()
	return managed
}

func (w *managedWorker) Stop(timeout time.Duration) error {
	w.cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-w.done:
		return w.err
	case <-timer.C:
		return errors.New("timed out waiting for outbox worker shutdown")
	}
}

func shutdownServer(server *http.Server, result <-chan error, logger *slog.Logger) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return err
	}
	err := <-result
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("API stopped gracefully")
	return nil
}
