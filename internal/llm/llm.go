package llm

import (
	"net/http"
	"os"
)

type LLM struct {
	apiKey string
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewLLM() *LLM {
	return &LLM{
		apiKey: os.Getenv("OPEN_ROUTER_KEY"),
	}
}

func (l *LLM) Chat(messages []ChatMessage) (string, error) {
