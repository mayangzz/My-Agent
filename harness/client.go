package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 是 DeepSeek(OpenAI 兼容)聊天接口的最小封装:只做"把消息+工具发出去,拿回一条回复"。
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

type chatRequest struct {
	Model    string       `json:"model"`
	Messages []Message    `json:"messages"`
	Tools    []ToolSchema `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调一次模型:发出当前对话 + 可用工具,返回模型这一轮的回复消息。
func (c *Client) Chat(ctx context.Context, msgs []Message, tools []ToolSchema) (Message, error) {
	const method = "Client.Chat"
	payload, err := json.Marshal(chatRequest{Model: c.Model, Messages: msgs, Tools: tools})
	if err != nil {
		return Message{}, fmt.Errorf("method=%s marshal: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Message{}, fmt.Errorf("method=%s new request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("method=%s do: %w", method, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return Message{}, fmt.Errorf("method=%s unmarshal (status=%d body=%s): %w", method, resp.StatusCode, body, err)
	}
	if out.Error != nil {
		return Message{}, fmt.Errorf("method=%s api error: %s", method, out.Error.Message)
	}
	if resp.StatusCode != http.StatusOK || len(out.Choices) == 0 {
		return Message{}, fmt.Errorf("method=%s bad response (status=%d body=%s)", method, resp.StatusCode, body)
	}
	return out.Choices[0].Message, nil
}
