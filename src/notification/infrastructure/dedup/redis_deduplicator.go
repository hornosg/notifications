package dedup

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisDeduplicator implementa port.Deduplicator sobre Redis con SET NX + TTL.
type RedisDeduplicator struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// NewRedisDeduplicator crea el deduplicador. ttl<=0 cae al default de 1h.
func NewRedisDeduplicator(client *redis.Client, ttl time.Duration) *RedisDeduplicator {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RedisDeduplicator{
		client: client,
		ttl:    ttl,
		prefix: "notif:dedup:",
	}
}

// MarkIfNew ejecuta SET key value NX EX ttl. true = la clave no existía (es nueva).
func (d *RedisDeduplicator) MarkIfNew(ctx context.Context, key string) (bool, error) {
	return d.client.SetNX(ctx, d.prefix+key, "1", d.ttl).Result()
}
