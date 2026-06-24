package harness

import "encoding/json"

// Message 是 OpenAI 兼容的对话消息;assistant 要调工具时带 ToolCalls,role=tool 的结果消息带 ToolCallID。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall 是模型"开口点单":它只说要调哪个工具、参数是什么,并不执行。
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // 模型返回的是 JSON 字符串
	} `json:"function"`
}

// ToolSchema 是发给模型的工具定义,告诉它有哪些工具、各要什么参数。
type ToolSchema struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// Tool = 工具定义 + 敏感度 + 真正执行它的 Go 函数(harness 的"手")。
type Tool struct {
	Def         FunctionDef
	Sensitivity string // read | write | exec —— 权限策略按它决定 allow/ask/deny
	Run         func(args json.RawMessage) (string, error)
}
