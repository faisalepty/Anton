package llm

import (
	"context"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)


type LLM struct {
	client openai.Client
}

func NewLLM(apiKey string) *LLM {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://openrouter.ai/api/v1"),
	)
	return &LLM{
		client: client,
	}
}

func (l *LLM) Invoke(Messages *[]openai.ChatCompletionMessageParamUnion) (string, error) {
	params := openai.ChatCompletionNewParams{
		Messages: *Messages,
		Model: "openai/gpt-oss-120b:free",
	}
	response, err := l.client.Chat.Completions.New(
		context.Background(),
		params,
	)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Message.Content, nil
}
