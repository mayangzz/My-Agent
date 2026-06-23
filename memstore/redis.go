package memstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mayangzz/My-Agent/harness"
	"github.com/redis/go-redis/v9"
)

const sessionTTL = 24 * time.Hour // 对话 24h 没动静自动过期

// Redis 把一个 session 的历史存成一个 List,适合"对话维度"的热会话,带 TTL 自动过期。
type Redis struct {
	rdb *redis.Client
}

func NewRedis(ctx context.Context, addr, password string) (*Redis, error) {
	const method = "memstore.NewRedis"
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("method=%s ping: %w", method, err)
	}
	return &Redis{rdb: rdb}, nil
}

func key(session string) string { return "agent:session:" + session }

func (r *Redis) Append(ctx context.Context, session string, msg harness.Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("method=Redis.Append marshal: %w", err)
	}
	k := key(session)
	if err := r.rdb.RPush(ctx, k, b).Err(); err != nil {
		return fmt.Errorf("method=Redis.Append rpush: %w", err)
	}
	r.rdb.Expire(ctx, k, sessionTTL) // 续期,过期即丢
	return nil
}

func (r *Redis) Load(ctx context.Context, session string) ([]harness.Message, error) {
	vals, err := r.rdb.LRange(ctx, key(session), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("method=Redis.Load lrange: %w", err)
	}
	out := make([]harness.Message, 0, len(vals))
	for _, v := range vals {
		var m harness.Message
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("method=Redis.Load unmarshal: %w", err)
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *Redis) Reset(ctx context.Context, session string) error {
	if err := r.rdb.Del(ctx, key(session)).Err(); err != nil {
		return fmt.Errorf("method=Redis.Reset del: %w", err)
	}
	return nil
}
