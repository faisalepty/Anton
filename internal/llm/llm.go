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

func New(apiKey string) *Client {
	return &Client{
		inner: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL("https://openrouter.ai/api/v1"), // https://router.huggingface.co/v1, https://openrouter.ai/api/v1
		),
	}
}

// ChatWithTools sends messages with optional tool schemas.
// Returns the full completion so callers can use msg.ToParam() and inspect tool calls.
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
	fmt.Print(resp)
	if err != nil {
		return nil, fmt.Errorf("LLM: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}
	return resp, nil
}
