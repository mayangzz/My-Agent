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
	"github.com/mayangzz/My-Agent/settings"
)

const (
	settingsPath = "settings.json"     // 可调设置(后台可改),可提交默认值
	secretsPath  = "config.local.json" // 密钥/连接串,gitignored,绝不提交
)

func main() {
	sec := loadSecrets()

	// 子命令:admin 起后台,其余进 REPL。
	args := os.Args[1:]
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

	agent := &harness.Agent{
		Client: client, Tools: reg, Memory: mem,
		Perms: harness.Perms(st.Permissions), Confirm: confirm,
		System: st.SystemPrompt, MaxSteps: st.MaxSteps,
	}

	const session = "cli" // 固定 session,跨轮(持久后端下还跨重启)记忆;/reset 清空
	fmt.Printf("My-Agent ready (model=%s, memory=%s). Type a task, /reset to clear, Ctrl-D to quit.\n", st.Model, backend)
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

// Secrets 是密钥/连接串,只从本地 config.local.json 读,绝不进 git。
type Secrets struct {
	DeepSeekAPIKey  string `json:"deepseek_api_key"`
	DeepSeekBaseURL string `json:"deepseek_base_url"`
	PostgresDSN     string `json:"postgres_dsn"`
	RedisAddr       string `json:"redis_addr"`
	RedisPassword   string `json:"redis_password"`
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
