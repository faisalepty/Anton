package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type chatRequest struct {
	Model   string       `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

func (l *LLM) request(messages []ChatMessage) (string, error) {
	reqBody := chatRequest{
		Model:   "gpt-4o",
		Messages: messages,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request body: %v", err)
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making API call: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from API")
	}
	return result.Choices[0].Message.Content, nil
}