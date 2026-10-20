// Package redisclient wraps the go-redis client and exposes a thin helper for
// cache operations used across the application (categories, banners, etc.).
package redisclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps *redis.Client with convenience helpers.
type Client struct {
	rdb *redis.Client
}

// Connect parses a redis URL and verifies connectivity.
// If Redis is unavailable, it returns a no-op client (nil rdb) so the app
// can still start without a Redis instance (dev/bypass mode).
func Connect(ctx context.Context, url string) (*Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(opt)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		// Redis not available — return a no-op client so the app starts anyway.
		_ = rdb.Close()
		return &Client{rdb: nil}, nil
	}
	return &Client{rdb: rdb}, nil
}

// Raw returns the underlying go-redis client for advanced use.
func (c *Client) Raw() *redis.Client { return c.rdb }

// Available reports whether a real Redis connection is present.
func (c *Client) Available() bool { return c != nil && c.rdb != nil }

// Close closes the connection.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// Health confirms Redis is reachable.
func (c *Client) Health(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil // no-op in bypass mode
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.rdb.Ping(ctx).Err()
}

// SetJSON marshals v to JSON and stores it with a TTL.
func (c *Client) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil // no-op in bypass mode
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, ttl).Err()
}

// GetJSON fetches and unmarshals a JSON value. Returns (false, nil) on miss.
func (c *Client) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil // cache miss in bypass mode
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

// Delete removes one or more keys (used for cache invalidation).
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if c == nil || c.rdb == nil {
		return nil // no-op in bypass mode
	}
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// DeleteByPattern removes all keys matching a glob pattern (SCAN-based).
func (c *Client) DeleteByPattern(ctx context.Context, pattern string) error {
	if c == nil || c.rdb == nil {
		return nil // no-op in bypass mode
	}
	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) >= 100 {
			if err := c.rdb.Del(ctx, batch...).Err(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(batch) > 0 {
		return c.rdb.Del(ctx, batch...).Err()
	}
	return nil
}
