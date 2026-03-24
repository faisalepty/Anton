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
	"pipeline/internal/skill"
)

func main() {
	model := flag.String("model", "openai/gpt-oss-120b:free", "OpenRouter model ID")
	agentsDir := flag.String("agents", "agent-definitions", "Path to agent definitions folder")
	skillsDir := flag.String("skills", "skills", "Path to skills folder")
	maxDepth := flag.Int("depth", 2, "Max sub-agent delegation depth")
	flag.Parse()

	// Require OpenRouter key
	apiKey := ""
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: OPENROUTER_API_KEY not set")
		os.Exit(1)
	}

	// // Warn about optional keys
	// if os.Getenv("TAVILY_API_KEY") == "" {
	// 	fmt.Fprintln(os.Stderr, "warning: TAVILY_API_KEY not set — researcher agent will not work")
	// 	fmt.Fprintln(os.Stderr, "         get a free key at https://app.tavily.com")
	// }

	// Load skills registry
	skillReg, err := skill.Load(*skillsDir, skill.Providers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading skills: %v\n", err)
		os.Exit(1)
	}

	// Load agent registry (agents declare which skills they use)
	reg, err := registry.Load(*agentsDir, skillReg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading agents: %v\n", err)
		os.Exit(1)
	}
	if len(reg.Names()) == 0 {
		fmt.Fprintf(os.Stderr, "error: no agents found in '%s'\n", *agentsDir)
		os.Exit(1)
	}

	client := llm.New(apiKey)
	mem := memory.New()
	p := planner.New(client, reg, *model, *maxDepth)

	// Startup banner
	fmt.Println()
	fmt.Println("Pipeline Agent")
	fmt.Printf("model     : %s\n", *model)
	fmt.Printf("agents    : %s  (%s)\n", *agentsDir, strings.Join(reg.Names(), ", "))
	fmt.Printf("skills    : %s\n", *skillsDir)
	fmt.Printf("max-depth : %d\n", *maxDepth)
	fmt.Println()
	fmt.Println("Commands: /agents  /skills  /model <n>  /clear  /exit")
	fmt.Println(strings.Repeat("─", 55))
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
			fmt.Println("Memory cleared.\n")

		case input == "/agents":
			fmt.Println()
			for _, name := range reg.Names() {
				def, _ := reg.Get(name)
				tools := strings.Join(def.ToolNames(), ", ")
				if tools == "" {
					tools = "none (pure LLM)"
				}
				skillList := strings.Join(def.SkillNames, ", ")
				if skillList == "" {
					skillList = "none"
				}
				fmt.Printf("  [%s]\n", name)
				fmt.Printf("    role  : %s\n", def.Role)
				fmt.Printf("    skills: %s\n", skillList)
				fmt.Printf("    tools : %s\n", tools)
				fmt.Println()
			}

		case input == "/skills":
			fmt.Println()
			fmt.Println("Loaded skills and their tools:")
			// Print from each agent's declared skills
			seen := make(map[string]bool)
			for _, name := range reg.Names() {
				def, _ := reg.Get(name)
				for _, sn := range def.SkillNames {
					if !seen[sn] {
						seen[sn] = true
						fmt.Printf("  [%s]\n", sn)
					}
				}
			}
			fmt.Println()

		case strings.HasPrefix(input, "/model "):
			m := strings.TrimSpace(strings.TrimPrefix(input, "/model "))
			if m == "" {
				fmt.Println("Usage: /model <model-id>")
				continue
			}
			p.SetModel(m)
			fmt.Printf("Model -> %s\n\n", m)

		default:
			fmt.Println()
			result, err := p.Run(context.Background(), input, mem)
			if err != nil {
				fmt.Printf("Error: %v\n\n", err)
				continue
			}
			fmt.Printf("\nAnswer:\n%s\n\n", result)
			fmt.Println(strings.Repeat("─", 55))
			fmt.Println()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scanner error: %v\n", err)
	}
}
