package config

import "testing"

func TestLoad(t *testing.T) {
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
