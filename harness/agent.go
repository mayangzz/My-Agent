package harness

import (
	"context"
	"fmt"
	"log"
)

// Agent 就是 harness 的灵魂:一个"调模型 → 执行工具 → 把结果塞回 → 再调"的循环。
type Agent struct {
	Client   *Client
	Tools    *Registry
	System   string
	MaxSteps int
}

// Run 跑一个任务直到模型给出最终答案(不再要工具)或撞到 MaxSteps 兜底。
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	const method = "Agent.Run"

	msgs := make([]Message, 0, 8)
	if a.System != "" {
		msgs = append(msgs, Message{Role: "system", Content: a.System})
	}
	msgs = append(msgs, Message{Role: "user", Content: userInput})

	for step := 1; step <= a.MaxSteps; step++ {
		reply, err := a.Client.Chat(ctx, msgs, a.Tools.Schemas()) // ① 调"大脑",带上工具定义
		if err != nil {
			return "", fmt.Errorf("method=%s step=%d: %w", method, step, err)
		}
		msgs = append(msgs, reply)

		if len(reply.ToolCalls) == 0 { // ② 模型不要工具了 → 最终答案
			return reply.Content, nil
		}

		for _, tc := range reply.ToolCalls { // ③ 模型要调工具(一轮可能多个)
			log.Printf("method=%s step=%d tool=%s args=%s", method, step, tc.Function.Name, tc.Function.Arguments)
			result := a.Tools.Exec(tc)                                                 // 真执行(harness 的"手")
			msgs = append(msgs, Message{Role: "tool", ToolCallID: tc.ID, Content: result}) // ④ 结果塞回上下文
		}
		// ⑤ 带着新结果再循环,让模型决定下一步
	}
	return "", fmt.Errorf("method=%s reached max steps=%d", method, a.MaxSteps)
}
