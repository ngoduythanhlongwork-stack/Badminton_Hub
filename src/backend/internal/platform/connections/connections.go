package connections

import (
	"context"
	"errors"
	"time"

	"badmintonhub/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Connections struct {
	Postgres *pgxpool.Pool
	Redis    *redis.Client
}

// Open validates client settings and verifies PostgreSQL before returning.
// Redis is optional acceleration; callers report its availability separately.
func Open(ctx context.Context, cfg config.Config) (*Connections, error) {
	pgConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	pgConfig.MaxConns = 10
	pgConfig.MinConns = 0
	pgConfig.MaxConnLifetime = time.Hour
	pgConfig.MaxConnIdleTime = 5 * time.Minute
	pgConfig.ConnConfig.ConnectTimeout = cfg.DependencyTimeout
	redisConfig, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, errors.New("invalid REDIS_URL")
	}
	redisConfig.DialTimeout = cfg.DependencyTimeout
	redisConfig.ReadTimeout = cfg.DependencyTimeout
	redisConfig.WriteTimeout = cfg.DependencyTimeout
	redisConfig.PoolTimeout = cfg.DependencyTimeout
	redisConfig.ContextTimeoutEnabled = true
	redisConfig.MaxRetries = -1
	redisConfig.PoolSize = 10
	pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		return nil, errors.New("cannot initialize PostgreSQL pool")
	}
	pingCtx, cancel := context.WithTimeout(ctx, cfg.DependencyTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		// Do not expose driver errors: they may include connection credentials.
		return nil, errors.New("PostgreSQL unavailable: check configuration and service")
	}
	return &Connections{Postgres: pool, Redis: redis.NewClient(redisConfig)}, nil
}

func (c *Connections) Close() {
	_ = c.Redis.Close()
	c.Postgres.Close()
}

func (c *Connections) PingRedis(ctx context.Context) error {
	return c.Redis.Ping(ctx).Err()
}
