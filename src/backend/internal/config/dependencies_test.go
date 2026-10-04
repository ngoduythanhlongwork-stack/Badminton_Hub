package config

import (
	"strings"
	"testing"
)

func TestInvalidDependencyConfiguration(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"DATABASE_URL", ""},
		{"DATABASE_URL", "https://user:supersecret@localhost"},
		{"DATABASE_URL", "postgres:///missing-host"},
		{"REDIS_URL", ""},
		{"REDIS_URL", "http://user:supersecret@localhost"},
		{"IDENTITY_TOKEN_SECRET", "too-short"},
		{"DEPENDENCY_TIMEOUT", "0s"},
		{"DEPENDENCY_TIMEOUT", "-1s"},
		{"DEPENDENCY_TIMEOUT", "31s"},
		{"DEPENDENCY_TIMEOUT", "invalid"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", "127.0.0.1:5080")
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv("REDIS_URL", "redis://localhost:6379/0")
			t.Setenv("IDENTITY_TOKEN_SECRET", "test_identity_token_secret_32_bytes_minimum")
			t.Setenv("DEPENDENCY_TIMEOUT", "2s")
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			if err == nil {
				t.Fatal("expected invalid configuration")
			}
			if strings.Contains(err.Error(), "supersecret") {
				t.Fatal("error leaks credentials")
			}
		})
	}
}
