// Package memstore 提供 harness.Memory 的几种实现,启动时按 backend 选一个。
package memstore

import (
	"context"
	"fmt"

	"github.com/mayangzz/My-Agent/harness"
)

// Config 是各后端的连接信息(密钥/连接串来自 config.local.env,不进 settings.json)。
type Config struct {
	Backend       string // inmem | postgres | redis
	PostgresDSN   string
	RedisAddr     string
	RedisPassword string
}

// New 按 backend 造一个 Memory 实现。
func New(ctx context.Context, cfg Config) (harness.Memory, error) {
	switch cfg.Backend {
	case "inmem":
		return NewInMem(), nil
	case "postgres":
		return NewPostgres(ctx, cfg.PostgresDSN)
	case "redis":
		return NewRedis(ctx, cfg.RedisAddr, cfg.RedisPassword)
	default:
		return nil, fmt.Errorf("method=memstore.New unknown backend %q (want inmem|postgres|redis)", cfg.Backend)
	}
}
