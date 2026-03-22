package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"pipeline/internal/llm"
	"pipeline/internal/memory"
	"pipeline/internal/planner"
	"pipeline/internal/registry"
)

func main() {
	model     := flag.String("model", "openai/gpt-oss-120b", "OpenRouter model ID")
	agentsDir := flag.String("agents", "agent-definitions", "Path to agent definitions folder")
	maxDepth  := flag.Int("depth", 2, "Max sub-agent delegation depth")
	flag.Parse()

	// Require API key
	apiKey := "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"
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: OPENROUTER_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Load all agent definitions from the agent-definitions folder
	reg, err := registry.Load(*agentsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading agents: %v\n", err)
		os.Exit(1)
	}
	if len(reg.Names()) == 0 {
		fmt.Fprintf(os.Stderr, "error: no agents found in '%s'\n", *agentsDir)
		os.Exit(1)
	}

	client := llm.New(apiKey)
	mem    := memory.New()
	p      := planner.New(client, reg, *model, *maxDepth)

	// Print startup banner
	fmt.Println()
	fmt.Println("Pipeline Agent")
	fmt.Printf("model     : %s\n", *model)
	fmt.Printf("agents dir: %s\n", *agentsDir)
	fmt.Printf("max depth : %d\n", *maxDepth)
	fmt.Printf("loaded    : %s\n", strings.Join(reg.Names(), ", "))
	fmt.Println()
	fmt.Println("Commands: /agents  /model <name>  /clear  /exit")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {

		case input == "/exit":
			fmt.Println("Goodbye.")
			return

		case input == "/clear":
			mem.Clear()
			fmt.Println("Memory cleared.")
			fmt.Println()

		case input == "/agents":
			fmt.Println()
			for _, name := range reg.Names() {
				def, _ := reg.Get(name)
				tools := strings.Join(def.ToolNames(), ", ")
				if tools == "" {
					tools = "none (pure LLM)"
				}
				fmt.Printf("  [%s]\n", name)
				fmt.Printf("    role : %s\n", def.Role)
				fmt.Printf("    tools: %s\n", tools)
				fmt.Println()
			}

		case strings.HasPrefix(input, "/model "):
			newModel := strings.TrimSpace(strings.TrimPrefix(input, "/model "))
			if newModel == "" {
				fmt.Println("Usage: /model <model-id>")
				continue
			}
			p.SetModel(newModel)
			fmt.Printf("Model switched to: %s\n\n", newModel)

		default:
			fmt.Println()
			result, err := p.Run(context.Background(), input, mem)
			if err != nil {
				fmt.Printf("Error: %v\n\n", err)
				continue
			}
			fmt.Printf("\nAnswer:\n%s\n\n", result)
			fmt.Println(strings.Repeat("─", 50))
			fmt.Println()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scanner error: %v\n", err)
	}
}