package storage

import (
	"context"
	"time"
)

type Storage interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	TTL(ctx context.Context, key string) (int, error)
	ExpireIn(ctx context.Context, key string, ttl time.Duration) (int, error)
	Delete(ctx context.Context, key string) error
}
