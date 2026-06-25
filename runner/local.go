// Package runner 提供 harness.Runner 的实现:子 agent 在哪儿跑。
// LocalRunner 在本进程内跑(无需 Docker),DockerRunner 跑进一次性容器(隔离)。
package runner

import (
	"context"
	"fmt"

	"github.com/mayangzz/My-Agent/harness"
)

// LocalRunner 在本进程内跑子 agent(goroutine 级,无需 Docker)。
// 子 agent 共用同一套模型客户端与同一个 Memory 后端(状态面共享),
// 但有独立 session 和角色化 system prompt——这就是"共享上下文、各司其职"的本地版。
type LocalRunner struct {
	// Build 按角色造一个子 agent(由 main 注入,持有 client / memory / 工具表等)。
	// 子 agent 的工具表不含 spawn_subagent,避免无限套娃。
	Build func(role string) *harness.Agent
}

func (r *LocalRunner) RunSubagent(ctx context.Context, role, task string) (string, error) {
	const method = "LocalRunner.RunSubagent"
	if r.Build == nil {
		return "", fmt.Errorf("method=%s: no agent builder configured", method)
	}
	return r.Build(role).Run(ctx, "sub:"+role, task)
}
