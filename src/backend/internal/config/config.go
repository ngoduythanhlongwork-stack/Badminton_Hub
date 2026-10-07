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
	Environment         string
	HTTPAddr            string
	PublicBaseURL       string
	DatabaseURL         string
	RedisURL            string
	IdentityTokenSecret string
	DependencyTimeout   time.Duration
}

func Load() (Config, error) {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if environment == "" {
		environment = "development"
	}
	if environment != "development" && environment != "test" && environment != "staging" && environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, staging, or production")
	}
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
	publicBaseURL := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:5080"
	}
	publicURL, err := url.Parse(publicBaseURL)
	if err != nil || publicURL.Host == "" || publicURL.User != nil || publicURL.RawQuery != "" || publicURL.Fragment != "" || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || strings.TrimRight(publicURL.Path, "/") != "" {
		return Config{}, fmt.Errorf("PUBLIC_BASE_URL must be an HTTP(S) origin without a path")
	}
	if environment == "production" {
		database, _ := url.Parse(databaseURL)
		if sslMode := database.Query().Get("sslmode"); sslMode != "require" && sslMode != "verify-ca" && sslMode != "verify-full" {
			return Config{}, fmt.Errorf("DATABASE_URL must enable TLS in production")
		}
		if publicURL.Scheme != "https" {
			return Config{}, fmt.Errorf("PUBLIC_BASE_URL must use HTTPS in production")
		}
		if strings.Contains(strings.ToLower(identityTokenSecret), "local_only") || strings.Contains(strings.ToLower(identityTokenSecret), "change_me") {
			return Config{}, fmt.Errorf("IDENTITY_TOKEN_SECRET must not use a development placeholder in production")
		}
	}
	timeout := 2 * time.Second
	if value := os.Getenv("DEPENDENCY_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 || parsed > 30*time.Second {
			return Config{}, fmt.Errorf("DEPENDENCY_TIMEOUT must be a duration greater than zero and at most 30s")
		}
		timeout = parsed
	}
	return Config{Environment: environment, HTTPAddr: addr, PublicBaseURL: strings.TrimRight(publicBaseURL, "/"), DatabaseURL: databaseURL, RedisURL: redisURL, IdentityTokenSecret: identityTokenSecret, DependencyTimeout: timeout}, nil
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
