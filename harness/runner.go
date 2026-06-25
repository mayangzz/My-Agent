package harness

import (
	"context"
	"encoding/json"
	"fmt"
)

// Runner 决定子 agent 在哪儿跑:本进程(LocalRunner)还是一次性容器(DockerRunner)。
// harness 只认这个接口,具体实现由 runner 包提供、启动时按配置注入——和 Memory 一个套路。
type Runner interface {
	RunSubagent(ctx context.Context, role, task string) (string, error)
}

// SpawnSubagentTool 把"派一个子 agent 干活"包成工具:主 agent 给出角色(各司其职的人设)
// 和一个自包含的子任务,harness 经 Runner 跑出一个专注的子 agent,把它的结果带回主上下文。
func SpawnSubagentTool(run Runner) Tool {
	return Tool{
		Def: FunctionDef{
			Name: "spawn_subagent",
			Description: "Delegate a focused subtask to a specialized sub-agent. Give it a role " +
				"(its persona/expertise) and a self-contained task; it works independently and returns its result.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"role": map[string]any{"type": "string", "description": "The sub-agent's role/expertise, e.g. 'code reviewer', 'researcher'."},
					"task": map[string]any{"type": "string", "description": "A self-contained task, including any context the sub-agent needs."},
				},
				"required": []string{"role", "task"},
			},
		},
		Sensitivity: "exec", // 派子 agent 会真的烧 token、跑工具,按 exec 过权限闸
		Run: func(args json.RawMessage) (string, error) {
			var p struct {
				Role string `json:"role"`
				Task string `json:"task"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("bad args: %w", err)
			}
			if p.Role == "" || p.Task == "" {
				return "", fmt.Errorf("role and task are required")
			}
			return run.RunSubagent(context.Background(), p.Role, p.Task)
		},
	}
}
