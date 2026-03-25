package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"

	"pipeline/internal/agent"
	"pipeline/internal/llm"
	"pipeline/internal/memory"
	"pipeline/internal/registry"
)

type PlanStep struct {
	Agent string `json:"agent"`
	Task  string `json:"task"`
}

type Planner struct {
	llm      *llm.Client
	registry *registry.Registry
	model    string
	maxDepth int
}

func New(client *llm.Client, reg *registry.Registry, model string, maxDepth int) *Planner {
	return &Planner{llm: client, registry: reg, model: model, maxDepth: maxDepth}
}

func (p *Planner) SetModel(model string) { p.model = model }

func (p *Planner) Run(ctx context.Context, goal string, mem *memory.Memory) (string, error) {
	fmt.Printf("[planner] goal: %s\n", goal)

	steps, err := p.makePlan(ctx, goal, mem.Summarize())
	if err != nil || len(steps) == 0 {
		fmt.Printf("[planner] planning failed (%v) — fallback\n", err)
		return p.runFallback(ctx, goal)
	}

	fmt.Printf("[planner] %d step(s):\n", len(steps))
	for i, s := range steps {
		fmt.Printf("  %d. [%s] %s\n", i+1, s.Agent, s.Task)
	}
	fmt.Println()

	var contextBuilder strings.Builder
	var results []string

	for i, step := range steps {
		def, ok := p.registry.Get(step.Agent)
		if !ok {
			fmt.Printf("[planner] unknown agent '%s' in step %d — skipping\n", step.Agent, i+1)
			continue
		}

		fullTask := step.Task
		if contextBuilder.Len() > 0 {
			fullTask = fmt.Sprintf(
				"RESEARCH FINDINGS FROM PREVIOUS STEPS — use ONLY this, do not use training knowledge:\n\n%s\n\n"+
					"YOUR TASK: %s\n\n"+
					"IMPORTANT: Base your response entirely on the findings above.",
				contextBuilder.String(), step.Task,
			)
		}

		fmt.Printf("[planner] -> [%s] step %d\n", step.Agent, i+1)
		worker := agent.New(def, p.llm, p.model, p.maxDepth)
		result, err := worker.Run(ctx, fullTask, 0)
		if err != nil {
			result = fmt.Sprintf("error: %v", err)
		}

		contextBuilder.WriteString(fmt.Sprintf("Step %d [%s]: %s\n-> %s\n\n", i+1, step.Agent, step.Task, result))
		results = append(results, fmt.Sprintf("Step %d [%s]: %s\nResult: %s", i+1, step.Agent, step.Task, result))
		mem.Add(step.Agent, step.Task, result)
		fmt.Println()
	}

	if len(results) == 0 {
		return "No steps executed.", nil
	}
	return p.synthesise(ctx, goal, results)
}

func (p *Planner) makePlan(ctx context.Context, goal, priorContext string) ([]PlanStep, error) {
	systemPrompt := fmt.Sprintf(
		"You are a task planner. Today is %s.\n\n"+
			"Available agents:\n%s\n\n"+
			"Return ONLY a JSON array. Each element:\n"+
			"  { \"agent\": \"<name>\", \"task\": \"<specific task>\" }\n\n"+
			"Rules:\n"+
			"- 1-4 steps. Simple requests = 1 step.\n"+
			"- Steps run in order — later steps can reference earlier results.\n"+
			"- Pick the most appropriate agent for each step.\n"+
			"- Return raw JSON only. No markdown, no explanation.\n\n"+
			"TASK WRITING RULES:\n"+
			"- writer task: always say 'summarise the research findings from the previous step'\n"+
			"- researcher task: include the current year in search queries\n\n"+
			"ROUTING:\n"+
			"  researcher      → web search, finding current facts\n"+
			"  writer          → drafting prose, summarising (does NOT write files)\n"+
			"  analyst         → reasoning over data, comparing options\n"+
			"  coder           → general coding tasks, scripts, full-stack\n"+
			"  coder-frontend  → UI, components, CSS, HTML, JS, React, Vue\n"+
			"  coder-backend   → APIs, servers, databases, auth, Go/Python/Node\n\n"+
			"NEVER:\n"+
			"- Add a separate file-saving step — coders save their own files\n"+
			"- Assign code execution to writer or researcher",
		time.Now().Format("Monday, January 2, 2006"),
		p.registry.Menu(),
	)

	userContent := goal
	if priorContext != "" {
		userContent = fmt.Sprintf("Prior context:\n%s\n\nNew request: %s", priorContext, goal)
	}

	resp, err := p.llm.ChatWithTools(ctx, p.model, []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userContent),
	}, nil)
	if err != nil {
		return nil, err
	}

	clean := strings.TrimSpace(resp.Choices[0].Message.Content)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var steps []PlanStep
	if err := json.Unmarshal([]byte(clean), &steps); err != nil {
		return nil, fmt.Errorf("parse plan: %w (raw: %q)", err, clean)
	}
	return steps, nil
}

func (p *Planner) synthesise(ctx context.Context, goal string, results []string) (string, error) {
	if len(results) == 1 {
		parts := strings.SplitN(results[0], "Result: ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1]), nil
		}
		return results[0], nil
	}

	resp, err := p.llm.ChatWithTools(ctx, p.model, []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("Synthesise multi-step agent results into one clear final answer."),
		openai.UserMessage(fmt.Sprintf(
			"Original request: %s\n\nStep results:\n%s\n\nFinal answer:",
			goal, strings.Join(results, "\n\n"),
		)),
	}, nil)
	if err != nil || len(resp.Choices) == 0 {
		return strings.Join(results, "\n\n"), nil
	}
	return resp.Choices[0].Message.Content, nil
}

func (p *Planner) runFallback(ctx context.Context, goal string) (string, error) {
	def := p.registry.First()
	if def == nil {
		return "", fmt.Errorf("no agents available")
	}
	worker := agent.New(def, p.llm, p.model, p.maxDepth)
	return worker.Run(ctx, goal, 0)
}
