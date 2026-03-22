package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"pipeline/internal/tool"
)

// Client wraps the official openai-go/v3 SDK pointed at OpenRouter.
type Client struct {
	inner openai.Client
}

// New creates a Client using the official SDK with OpenRouter as the base URL.
func New(apiKey string) *Client {
	inner := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://router.huggingface.co/v1"),
	)
	return &Client{inner: inner}
}

// Chat sends a plain conversation (no tools) and returns the text response.
func (c *Client) Chat(
	ctx context.Context,
	model string,
	messages []openai.ChatCompletionMessageParamUnion,
) (string, error) {
	resp, err := c.inner.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(model),
		Messages:  messages,
		MaxTokens: openai.Int(2048),
	})
	if err != nil {
		return "", fmt.Errorf("LLM chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

// ChatWithTools sends messages with optional tool schemas.
// Returns (textResponse, completion, error).
// The full completion is returned so the caller can use .ToParam() on the message.
// If the model produced tool calls, textResponse is empty.
// If the model produced a final answer, ToolCalls in the completion will be empty.
func (c *Client) ChatWithTools(
	ctx context.Context,
	model string,
	messages []openai.ChatCompletionMessageParamUnion,
	schemas []tool.Schema,
) (*openai.ChatCompletion, error) {

	params := openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(model),
		Messages:  messages,
		MaxTokens: openai.Int(2048),
	}

	// Build SDK tool params from our internal schema type.
	// Use ChatCompletionToolParam directly — the canonical v3 type.
	if len(schemas) > 0 {
		tools := make([]openai.ChatCompletionToolUnionParam, len(schemas))
		for i, s := range schemas {
			tools[i] = openai.ChatCompletionToolUnionParam{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name:        s.Function.Name,
						Description: openai.String(s.Function.Description),
						Parameters:  openai.FunctionParameters(s.Function.Parameters),
					},
				},
			}
		}
		params.Tools = tools
	}

	resp, err := c.inner.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("LLM chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}
	return resp, nil
}