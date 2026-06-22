package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mayangzz/My-Agent/harness"
)

const systemPrompt = `You are a small agent running inside a minimal harness.
Use the provided tools when they help answer the user. Think step by step.
When you have the final answer, reply directly without calling a tool.`

func main() {
	cfg := loadConfig()

	client := harness.NewClient(cfg["DEEPSEEK_BASE_URL"], cfg["DEEPSEEK_API_KEY"], cfg["DEEPSEEK_MODEL"])

	reg := harness.NewRegistry()
	reg.Add(harness.NowTool())
	reg.Add(harness.ReadFileTool())

	agent := &harness.Agent{Client: client, Tools: reg, System: systemPrompt, MaxSteps: 8}

	fmt.Printf("mini-harness ready (model=%s). Type a task, Ctrl-D to quit.\n", cfg["DEEPSEEK_MODEL"])
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		fmt.Print("\nyou> ")
		if !sc.Scan() {
			return
		}
		input := strings.TrimSpace(sc.Text())
		if input == "" {
			continue
		}
		answer, err := agent.Run(context.Background(), input)
		if err != nil {
			log.Printf("method=main run error: %v", err)
			continue
		}
		fmt.Printf("\nagent> %s\n", answer)
	}
}

// loadConfig 读取同目录的 config.local.env(key=val),环境变量优先;缺关键字段直接退出。
func loadConfig() map[string]string {
	cfg := map[string]string{
		"DEEPSEEK_BASE_URL": "https://api.deepseek.com",
		"DEEPSEEK_MODEL":    "deepseek-v4-pro",
	}
	if b, err := os.ReadFile("config.local.env"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				cfg[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	for _, k := range []string{"DEEPSEEK_API_KEY", "DEEPSEEK_BASE_URL", "DEEPSEEK_MODEL"} {
		if v := os.Getenv(k); v != "" {
			cfg[k] = v
		}
	}
	if cfg["DEEPSEEK_API_KEY"] == "" {
		log.Fatal("method=loadConfig missing DEEPSEEK_API_KEY (copy config.example.env to config.local.env and fill it)")
	}
	return cfg
}
