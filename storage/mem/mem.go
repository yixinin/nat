package mem

import (
	"context"
	"errors"
	"nat/storage"
	"time"

	"github.com/patrickmn/go-cache"
)

var ErrorNotFound = errors.New("not found")

type MemCache struct {
	c *cache.Cache
}

func NewMemCahce() storage.Storage {
	return &MemCache{
		c: cache.New(0, time.Minute),
	}
}
func (c *MemCache) Get(ctx context.Context, key string) (any, error) {
	val, ok := c.c.Get(key)
	if !ok {
		return nil, ErrorNotFound
	}
	return val, nil
}
func (c *MemCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	c.c.Set(key, val, ttl)
	return nil
}
func (c *MemCache) Replace(ctx context.Context, key string, val any, ttl time.Duration) error {
	c.c.Replace(key, val, ttl)
	return nil
}
func (c *MemCache) TTL(ctx context.Context, key string) (int, error) {
	_, t, ok := c.c.GetWithExpiration(key)
	if !ok {
		return -2, nil
	}
	if t.IsZero() {
		return -1, nil
	}
	return int(time.Until(t)), nil
}
func (c *MemCache) ExpireIn(ctx context.Context, key string, ttl time.Duration) (int, error) {
	val, ok := c.c.Get(key)
	if !ok {
		return -1, nil
	}
	err := c.c.Replace(key, val, ttl)
	return 0, err
}
func (c *MemCache) Delete(ctx context.Context, key string) error {
	c.c.Delete(key)
	return nil
}
