package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mayangzz/My-Agent/admin"
	"github.com/mayangzz/My-Agent/harness"
	"github.com/mayangzz/My-Agent/memstore"
	"github.com/mayangzz/My-Agent/runner"
	"github.com/mayangzz/My-Agent/settings"
)

const (
	settingsPath = "settings.json"     // 可调设置(后台可改),可提交默认值
	secretsPath  = "config.local.json" // 密钥/连接串,gitignored,绝不提交
)

func main() {
	args := os.Args[1:]

	// subagent 子命令:一次性跑一个子 agent(容器内由 DockerRunner 调起)。
	// 它从 env 读密钥(容器里没有 config.local.json),结果打到 stdout、日志走 stderr。
	if len(args) > 0 && args[0] == "subagent" {
		runSubagent(args[1:])
		return
	}

	sec := loadSecrets()

	// admin 起本地后台,其余进 REPL。
	if len(args) > 0 && args[0] == "admin" {
		if err := admin.Serve("127.0.0.1:7788", settingsPath); err != nil {
			log.Fatalf("method=main admin: %v", err)
		}
		return
	}

	fs := flag.NewFlagSet("repl", flag.ExitOnError)
	memOverride := fs.String("memory", "", "覆盖记忆后端: inmem|postgres|redis")
	fs.Parse(args)

	st, err := settings.Load(settingsPath)
	if err != nil {
		log.Fatalf("method=main load settings: %v", err)
	}

	backend := st.Memory.Backend
	if *memOverride != "" {
		backend = *memOverride
	}

	ctx := context.Background()
	mem, err := memstore.New(ctx, memstore.Config{
		Backend:       backend,
		PostgresDSN:   sec.PostgresDSN,
		RedisAddr:     sec.RedisAddr,
		RedisPassword: sec.RedisPassword,
	})
	if err != nil {
		log.Fatalf("method=main memory: %v", err)
	}
	if backend == "inmem" {
		log.Printf("method=main memory=inmem warning: restart loses all conversation memory")
	}

	client := harness.NewClient(sec.DeepSeekBaseURL, sec.DeepSeekAPIKey, st.Model)
	reg := harness.NewRegistry()
	reg.Add(harness.NowTool())
	reg.Add(harness.ReadFileTool())

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	confirm := func(prompt string) bool { // ask 策略下在 REPL 里征求同意
		fmt.Printf("%s (y/N) ", prompt)
		if !sc.Scan() {
			return false
		}
		ans := strings.ToLower(strings.TrimSpace(sc.Text()))
		return ans == "y" || ans == "yes"
	}

	// 子 agent 工厂:按角色造一个专注的子 agent,共用同一 client/memory(状态面共享),
	// 工具表不含 spawn_subagent —— 避免子 agent 再派子 agent 无限套娃。
	buildSub := func(role string) *harness.Agent {
		subReg := harness.NewRegistry()
		subReg.Add(harness.NowTool())
		subReg.Add(harness.ReadFileTool())
		return &harness.Agent{
			Client: client, Tools: subReg, Memory: mem,
			Perms: harness.Perms(st.Permissions), Confirm: confirm,
			System: subagentSystem(role), MaxSteps: st.MaxSteps,
		}
	}

	// 按 runner.mode 选执行后端:local 本进程,docker 一次性容器(需 config.local.json 给 docker 地址)。
	var run harness.Runner
	switch st.Runner.Mode {
	case "docker":
		if sec.DockerImage == "" {
			log.Fatalf("method=main runner=docker but docker_image is empty in %s", secretsPath)
		}
		run = &runner.DockerRunner{
			Image: sec.DockerImage, Host: sec.DockerHost,
			APIKey: sec.DeepSeekAPIKey, BaseURL: sec.DeepSeekBaseURL, Model: st.Model,
		}
	default:
		run = &runner.LocalRunner{Build: buildSub}
	}
	reg.Add(harness.SpawnSubagentTool(run))

	agent := &harness.Agent{
		Client: client, Tools: reg, Memory: mem,
		Perms: harness.Perms(st.Permissions), Confirm: confirm,
		System: st.SystemPrompt, MaxSteps: st.MaxSteps,
	}

	const session = "cli" // 固定 session,跨轮(持久后端下还跨重启)记忆;/reset 清空
	fmt.Printf("My-Agent ready (model=%s, memory=%s, runner=%s). Type a task, /reset to clear, Ctrl-D to quit.\n", st.Model, backend, st.Runner.Mode)
	for {
		fmt.Print("\nyou> ")
		if !sc.Scan() {
			return
		}
		input := strings.TrimSpace(sc.Text())
		if input == "" {
			continue
		}
		if input == "/reset" {
			if err := mem.Reset(ctx, session); err != nil {
				log.Printf("method=main reset: %v", err)
			} else {
				fmt.Println("(memory cleared)")
			}
			continue
		}
		answer, err := agent.Run(ctx, session, input)
		if err != nil {
			log.Printf("method=main run error: %v", err)
			continue
		}
		fmt.Printf("\nagent> %s\n", answer)
	}
}

// Secrets 是密钥/连接串/基础设施地址,只从本地 config.local.json 读,绝不进 git。
type Secrets struct {
	DeepSeekAPIKey  string `json:"deepseek_api_key"`
	DeepSeekBaseURL string `json:"deepseek_base_url"`
	PostgresDSN     string `json:"postgres_dsn"`
	RedisAddr       string `json:"redis_addr"`
	RedisPassword   string `json:"redis_password"`
	DockerHost      string `json:"docker_host"`  // runner=docker 时的 DOCKER_HOST,空则本机
	DockerImage     string `json:"docker_image"` // runner=docker 时跑子 agent 的镜像
}

func loadSecrets() Secrets {
	sec := Secrets{ // 默认值,缺字段不至于为空
		DeepSeekBaseURL: "https://api.deepseek.com",
		PostgresDSN:     "postgres://localhost:5432/myagent?sslmode=disable",
		RedisAddr:       "127.0.0.1:6379",
	}
	b, err := os.ReadFile(secretsPath)
	if err != nil {
		log.Fatalf("method=loadSecrets read %s: %v (copy config.example.json to %s and fill it)", secretsPath, err, secretsPath)
	}
	if err := json.Unmarshal(b, &sec); err != nil {
		log.Fatalf("method=loadSecrets parse %s: %v", secretsPath, err)
	}
	if sec.DeepSeekAPIKey == "" {
		log.Fatalf("method=loadSecrets missing deepseek_api_key in %s", secretsPath)
	}
	return sec
}

// subagentSystem 把角色拼成子 agent 的 system prompt(各司其职的人设)。
func subagentSystem(role string) string {
	if role == "" {
		role = "a focused assistant"
	}
	return "You are " + role + ", a sub-agent spawned to handle one specific task.\n" +
		"Use the provided tools when helpful. Stay focused on the task; when done, reply with the result directly."
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// runSubagent 一次性跑一个子 agent:密钥从 env 读、记忆用 inmem、答案打到 stdout。
// DockerRunner 在容器内调起的就是它。
func runSubagent(args []string) {
	const method = "runSubagent"
	fs := flag.NewFlagSet("subagent", flag.ExitOnError)
	role := fs.String("role", "", "sub-agent role / persona")
	task := fs.String("task", "", "the self-contained task to perform")
	fs.Parse(args)
	if *task == "" {
		log.Fatalf("method=%s missing --task", method)
	}
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		log.Fatalf("method=%s missing DEEPSEEK_API_KEY env", method)
	}

	client := harness.NewClient(envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com"), key, envOr("DEEPSEEK_MODEL", "deepseek-v4-pro"))
	reg := harness.NewRegistry()
	reg.Add(harness.NowTool())
	reg.Add(harness.ReadFileTool())

	ctx := context.Background()
	mem, err := memstore.New(ctx, memstore.Config{Backend: "inmem"}) // 容器内自包含,不连主进程记忆
	if err != nil {
		log.Fatalf("method=%s memory: %v", method, err)
	}
	agent := &harness.Agent{
		Client: client, Tools: reg, Memory: mem,
		Perms:  harness.Perms{"read": harness.Allow, "write": harness.Allow, "exec": harness.Allow}, // 容器内沙箱,放行
		System: subagentSystem(*role), MaxSteps: 6,
	}
	answer, err := agent.Run(ctx, "subagent", *task)
	if err != nil {
		log.Fatalf("method=%s run: %v", method, err)
	}
	fmt.Println(answer)
}
