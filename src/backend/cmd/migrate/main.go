package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"badmintonhub/internal/platform/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer pool.Close()
	runner, err := migrations.NewRunner(pool)
	if err != nil {
		return err
	}
	if err := runner.Up(ctx); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(os.Stdout, "migrations applied")
	return nil
}
