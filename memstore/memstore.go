// Package memstore 提供 harness.Memory 的几种实现,启动时按 backend 选一个。
package memstore

import (
	"context"
	"fmt"

	"github.com/mayangzz/My-Agent/harness"
)

// Config 是各后端要的信息:连接串/密钥来自 config.local.json,文件目录来自 settings.json。
type Config struct {
	Backend       string // none | inmem | filewiki | postgres | redis
	FileDir       string // filewiki 的存储目录
	PostgresDSN   string
	RedisAddr     string
	RedisPassword string
}

// New 按 backend 造一个 Memory 实现。
func New(ctx context.Context, cfg Config) (harness.Memory, error) {
	switch cfg.Backend {
	case "none":
		return None{}, nil
	case "inmem":
		return NewInMem(), nil
	case "filewiki", "file":
		return NewFileWiki(cfg.FileDir)
	case "postgres":
		return NewPostgres(ctx, cfg.PostgresDSN)
	case "redis":
		return NewRedis(ctx, cfg.RedisAddr, cfg.RedisPassword)
	default:
		return nil, fmt.Errorf("method=memstore.New unknown backend %q (want none|inmem|filewiki|postgres|redis)", cfg.Backend)
	}
}
