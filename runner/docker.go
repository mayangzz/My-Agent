package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DockerRunner 把每个子 agent 跑进一个一次性容器(--rm),做 blast-radius 隔离。
// 容器内执行 `my-agent subagent`:自带 inmem 记忆、经网络访问模型,结果从 stdout 带回。
// 需要 Docker——本机 daemon 或远程 DOCKER_HOST 地址(在 config.local.json 给)。
//
// 隔离边界:容器只挡住"本地破坏"(乱写文件/乱跑命令),挡不住对外副作用
// (调外部 API、发消息),那些仍靠权限闸。子 agent 拿不到主进程的 Memory,
// 上下文靠 task 注入——这正是"隔离计算 + 任务自包含"的取舍。
type DockerRunner struct {
	Image   string // 含 my-agent 二进制的镜像
	Host    string // DOCKER_HOST;空则用本机默认 daemon
	APIKey  string
	BaseURL string
	Model   string
}

func (r *DockerRunner) RunSubagent(ctx context.Context, role, task string) (string, error) {
	const method = "DockerRunner.RunSubagent"
	if r.Image == "" {
		return "", fmt.Errorf("method=%s: docker_image not set in config.local.json", method)
	}
	args := []string{
		"run", "--rm",
		"-e", "DEEPSEEK_API_KEY=" + r.APIKey,
		"-e", "DEEPSEEK_BASE_URL=" + r.BaseURL,
		"-e", "DEEPSEEK_MODEL=" + r.Model,
		r.Image,
		"/app/my-agent", "subagent", "--role", role, "--task", task,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	if r.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+r.Host)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("method=%s docker run: %w (stderr: %s)", method, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil // 容器把最终答案打到 stdout,日志走 stderr
}
