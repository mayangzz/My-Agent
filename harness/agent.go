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
	Memory   Memory
	Perms    Perms                     // 按敏感度的权限策略
	Confirm  func(prompt string) bool  // ask 时怎么征求同意;为 nil 则 ask 一律拒
	System   string
	MaxSteps int
}

// execTool 在执行工具前过一道权限闸:按敏感度 allow / ask / deny。
func (a *Agent) execTool(tc ToolCall) string {
	const method = "Agent.execTool"
	sens := a.Tools.Sensitivity(tc.Function.Name)
	switch a.Perms.Action(sens) {
	case Deny:
		log.Printf("method=%s tool=%s sensitivity=%s decision=deny", method, tc.Function.Name, sens)
		return fmt.Sprintf("error: tool %q denied by policy (sensitivity=%s)", tc.Function.Name, sens)
	case Ask:
		prompt := fmt.Sprintf("允许执行 %s [%s]? 参数: %s", tc.Function.Name, sens, tc.Function.Arguments)
		if a.Confirm == nil || !a.Confirm(prompt) {
			log.Printf("method=%s tool=%s decision=denied-by-user", method, tc.Function.Name)
			return fmt.Sprintf("error: tool %q denied by user", tc.Function.Name)
		}
	}
	return a.Tools.Exec(tc) // allow / 已确认 → 真执行
}

// Run 跑一个任务:先从 Memory 取回该 session 的历史,循环直到模型给最终答案或撞到 MaxSteps。
// 每条新消息(user / assistant / tool)都落进 Memory,于是跨轮自动有记忆。
func (a *Agent) Run(ctx context.Context, session, userInput string) (string, error) {
	const method = "Agent.Run"

	history, err := a.Memory.Load(ctx, session)
	if err != nil {
		return "", fmt.Errorf("method=%s load memory: %w", method, err)
	}

	// system 不入库,每轮从设置里现拼,改提示词不污染历史。
	msgs := make([]Message, 0, len(history)+4)
	if a.System != "" {
		msgs = append(msgs, Message{Role: "system", Content: a.System})
	}
	msgs = append(msgs, history...)

	user := Message{Role: "user", Content: userInput}
	msgs = append(msgs, user)
	if err := a.Memory.Append(ctx, session, user); err != nil {
		return "", fmt.Errorf("method=%s append user: %w", method, err)
	}

	for step := 1; step <= a.MaxSteps; step++ {
		reply, err := a.Client.Chat(ctx, msgs, a.Tools.Schemas()) // ① 调"大脑",带上工具定义
		if err != nil {
			return "", fmt.Errorf("method=%s step=%d: %w", method, step, err)
		}
		msgs = append(msgs, reply)
		if err := a.Memory.Append(ctx, session, reply); err != nil {
			return "", fmt.Errorf("method=%s append reply: %w", method, err)
		}

		if len(reply.ToolCalls) == 0 { // ② 模型不要工具了 → 最终答案
			return reply.Content, nil
		}

		for _, tc := range reply.ToolCalls { // ③ 模型要调工具(一轮可能多个)
			log.Printf("method=%s step=%d tool=%s args=%s", method, step, tc.Function.Name, tc.Function.Arguments)
			result := a.execTool(tc) // 过权限闸后执行
			toolMsg := Message{Role: "tool", ToolCallID: tc.ID, Content: result}
			msgs = append(msgs, toolMsg) // ④ 结果塞回上下文
			if err := a.Memory.Append(ctx, session, toolMsg); err != nil {
				return "", fmt.Errorf("method=%s append tool: %w", method, err)
			}
		}
		// ⑤ 带着新结果再循环,让模型决定下一步
	}
	return "", fmt.Errorf("method=%s reached max steps=%d", method, a.MaxSteps)
}
