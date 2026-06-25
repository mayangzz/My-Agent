package memstore

import (
	"context"

	"github.com/mayangzz/My-Agent/harness"
)

// None 是"不做记忆":Append 丢弃、Load 永远空。每轮对话都是干净的、没有跨轮上下文。
type None struct{}

func (None) Append(context.Context, string, harness.Message) error  { return nil }
func (None) Load(context.Context, string) ([]harness.Message, error) { return nil, nil }
func (None) Reset(context.Context, string) error                     { return nil }
