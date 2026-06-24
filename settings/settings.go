// Package settings 管理可调设置(非密钥),持久化到 settings.json,可被本地后台读写。
// 密钥/连接串不在这里——那些只在 config.local.env。
package settings

import (
	"encoding/json"
	"fmt"
	"os"
)

type Memory struct {
	Backend string `json:"backend"` // inmem | postgres | redis
}

type Settings struct {
	SystemPrompt string            `json:"system_prompt"`
	Model        string            `json:"model"`
	MaxSteps     int               `json:"max_steps"`
	Memory       Memory            `json:"memory"`
	Permissions  map[string]string `json:"permissions"` // 工具敏感度 -> allow|ask|deny
}

func Default() *Settings {
	return &Settings{
		SystemPrompt: "You are a small agent running inside a minimal harness.\n" +
			"Use the provided tools when they help answer the user. Think step by step.\n" +
			"When you have the final answer, reply directly without calling a tool.",
		Model:    "deepseek-v4-pro",
		MaxSteps: 8,
		Memory:   Memory{Backend: "postgres"},
		Permissions: map[string]string{
			"read":  "allow", // 只读默认放行
			"write": "ask",   // 写默认问
			"exec":  "ask",   // 执行默认问
		},
	}
}

// Load 读取 settings.json;文件不存在则写一份默认值再返回。
func Load(path string) (*Settings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		s := Default()
		return s, s.Save(path)
	}
	if err != nil {
		return nil, fmt.Errorf("method=settings.Load read: %w", err)
	}
	s := Default() // 以默认值打底,缺字段不至于为空
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("method=settings.Load unmarshal: %w", err)
	}
	return s, nil
}

func (s *Settings) Save(path string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("method=settings.Save marshal: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("method=settings.Save write: %w", err)
	}
	return nil
}
