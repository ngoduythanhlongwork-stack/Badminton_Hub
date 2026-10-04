package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains only settings consumed by this foundation.
type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	RedisURL            string
	IdentityTokenSecret string
	DependencyTimeout   time.Duration
}

func Load() (Config, error) {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:5080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR port must be between 1 and 65535")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if !validURL(databaseURL, "postgres", "postgresql") {
		return Config{}, fmt.Errorf("DATABASE_URL must be a PostgreSQL URL with a host")
	}
	if !validURL(redisURL, "redis", "rediss") {
		return Config{}, fmt.Errorf("REDIS_URL must be a Redis URL with a host")
	}
	identityTokenSecret := os.Getenv("IDENTITY_TOKEN_SECRET")
	if len(identityTokenSecret) < 32 {
		return Config{}, fmt.Errorf("IDENTITY_TOKEN_SECRET must contain at least 32 bytes")
	}
	timeout := 2 * time.Second
	if value := os.Getenv("DEPENDENCY_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 || parsed > 30*time.Second {
			return Config{}, fmt.Errorf("DEPENDENCY_TIMEOUT must be a duration greater than zero and at most 30s")
		}
		timeout = parsed
	}
	return Config{HTTPAddr: addr, DatabaseURL: databaseURL, RedisURL: redisURL, IdentityTokenSecret: identityTokenSecret, DependencyTimeout: timeout}, nil
}

func validURL(value string, schemes ...string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, scheme := range schemes {
		if u.Scheme == scheme {
			return true
		}
	}
	return false
}
