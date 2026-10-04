package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"badmintonhub/internal/modules/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "admin bootstrap failed:", err)
		os.Exit(1)
	}
	_, _ = fmt.Fprintln(os.Stdout, "initial admin permissions granted")
}

func run(ctx context.Context) error {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	accountID := strings.TrimSpace(os.Getenv("ADMIN_ACCOUNT_ID"))
	rationale := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_RATIONALE"))
	secret := os.Getenv("IDENTITY_TOKEN_SECRET")
	if databaseURL == "" || accountID == "" || rationale == "" || len(secret) < 32 {
		return fmt.Errorf("DATABASE_URL, ADMIN_ACCOUNT_ID, ADMIN_BOOTSTRAP_RATIONALE, and IDENTITY_TOKEN_SECRET (32+ bytes) are required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer pool.Close()
	service, err := identity.NewService(identity.Config{Pool: pool, TokenSecret: []byte(secret)})
	if err != nil {
		return err
	}
	return service.BootstrapAdmin(ctx, identity.BootstrapAdminRequest{AccountID: accountID, Rationale: rationale})
}
