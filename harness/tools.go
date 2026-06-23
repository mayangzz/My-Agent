package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"unicode/utf8"
)

// Registry 是工具注册表:按名字存工具,既能导出 schema 给模型,也能按名字执行。
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Add(t Tool) {
	r.tools[t.Def.Name] = t
}

// Schemas 导出所有工具定义,随每次请求发给模型。
func (r *Registry) Schemas() []ToolSchema {
	out := make([]ToolSchema, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, ToolSchema{Type: "function", Function: t.Def})
	}
	return out
}

// Exec 执行一次工具调用;工具不存在或报错都转成文本结果喂回模型,让它能自我纠错。
func (r *Registry) Exec(tc ToolCall) string {
	t, ok := r.tools[tc.Function.Name]
	if !ok {
		return "error: unknown tool " + tc.Function.Name
	}
	out, err := t.Run(json.RawMessage(tc.Function.Arguments))
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

// ---- 示例工具 ----

// NowTool 返回当前时间,演示一个零参数工具。
func NowTool() Tool {
	return Tool{
		Def: FunctionDef{
			Name:        "now",
			Description: "Return the current local date and time.",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
		Run: func(json.RawMessage) (string, error) {
			return time.Now().Format("2006-01-02 15:04:05 Mon"), nil
		},
	}
}

// ReadFileTool 读取一个文本文件,演示一个带参数的工具。
func ReadFileTool() Tool {
	return Tool{
		Def: FunctionDef{
			Name:        "read_file",
			Description: "Read a UTF-8 text file and return its content.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Absolute path of the file to read."},
				},
				"required": []string{"path"},
			},
		},
		Run: func(args json.RawMessage) (string, error) {
			var p struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("bad args: %w", err)
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", err
			}
			const maxBytes = 8000
			if len(b) > maxBytes { // 截断,别把上下文撑爆
				cut := maxBytes
				for cut > 0 && !utf8.RuneStart(b[cut]) { // 退到 rune 边界,别切断多字节字符(如中文)
					cut--
				}
				b = append(b[:cut:cut], []byte("\n...(truncated)")...)
			}
			return string(b), nil
		},
	}
}
