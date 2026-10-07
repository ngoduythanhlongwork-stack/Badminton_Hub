package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PUBLIC_BASE_URL", "http://localhost:5080")
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/test")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("IDENTITY_TOKEN_SECRET", "test_identity_token_secret_32_bytes_minimum")
	t.Setenv("DEPENDENCY_TIMEOUT", "")
	for _, tc := range []struct {
		value, want string
		invalid     bool
	}{
		{"", "127.0.0.1:5080", false},
		{":8080", ":8080", false},
		{"[::1]:5080", "[::1]:5080", false},
		{"invalid", "", true},
		{":0", "", true},
		{":65536", "", true},
		{":http", "", true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", tc.value)
			cfg, err := Load()
			if (err != nil) != tc.invalid {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.HTTPAddr != tc.want {
				t.Fatalf("address = %q, want %q", cfg.HTTPAddr, tc.want)
			}
		})
	}
}

func TestProductionRejectsUnsafeConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":5080")
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/test?sslmode=disable")
	t.Setenv("REDIS_URL", "rediss://redis.example.com:6379/0")
	t.Setenv("PUBLIC_BASE_URL", "https://api.example.com")
	t.Setenv("IDENTITY_TOKEN_SECRET", "independent_random_secret_more_than_32_bytes")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted PostgreSQL TLS disabled")
	}
	t.Setenv("DATABASE_URL", "postgres://user:password@db.example.com:5432/test?sslmode=require")
	t.Setenv("PUBLIC_BASE_URL", "http://api.example.com")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted an HTTP public origin")
	}
	t.Setenv("PUBLIC_BASE_URL", "https://api.example.com")
	t.Setenv("IDENTITY_TOKEN_SECRET", "local_only_change_me_identity_token_secret")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted a placeholder secret")
	}
	t.Setenv("IDENTITY_TOKEN_SECRET", "independent_random_secret_more_than_32_bytes")
	if _, err := Load(); err != nil {
		t.Fatalf("valid production configuration rejected: %v", err)
	}
}
