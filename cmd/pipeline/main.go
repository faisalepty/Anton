package main

import (
	"context"
	"fmt"
	"os"

	"pipeline/internal/agent"
	"pipeline/internal/llm"
	"pipeline/internal/registry"
)

func main() {
	key := os.Getenv("OPENROUTER_API_KEY")
	// key := os.Getenv("HUGGINGFACE_API_KEY")
	client := llm.New(key)

	agents, _, _ := registry.Load("./agent-definitions", "./skills")

	brain := agent.NewBrain(client, "openai/gpt-oss-120b:free", agents, 0)

	res, err := brain.Execute(context.Background(), "manager", "Check for the files in the current dir and summarize them.")
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}

	fmt.Println("\nRESULT:", res)
}
