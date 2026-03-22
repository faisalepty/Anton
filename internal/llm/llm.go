package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"pipeline/internal/tool"
)

const baseURL = "https://openrouter.ai/api/v1/chat/completions"

// Message is a single conversation turn.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall holds the tool name and JSON-encoded arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Client wraps the OpenRouter HTTP API.
type Client struct {
	apiKey string
	http   *http.Client
}

// New creates a new LLM client.
func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{},
	}
}

// Chat sends messages and returns a plain text response.
func (c *Client) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	text, _, err := c.ChatWithTools(ctx, model, messages, nil)
	return text, err
}

// ChatWithTools sends messages with optional tool schemas.
// Returns (textResponse, toolCalls, error).
// If the model wants to call tools, textResponse is empty and toolCalls is populated.
// If the model produces a final answer, textResponse is populated and toolCalls is empty.
func (c *Client) ChatWithTools(
	ctx context.Context,
	model string,
	messages []Message,
	schemas []tool.Schema,
) (string, []ToolCall, error) {

	reqBody := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": 2048,
	}
	if len(schemas) > 0 {
		reqBody["tools"] = schemas
		reqBody["tool_choice"] = "auto"
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("API error %d: %s", resp.StatusCode, rawBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", nil, fmt.Errorf("parse response: %w", err)
	}
	if result.Error != nil {
		return "", nil, fmt.Errorf("LLM error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("empty response from API")
	}

	msg := result.Choices[0].Message
	return msg.Content, msg.ToolCalls, nil
}