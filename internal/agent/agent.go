package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"

	"pipeline/internal/llm"
	"pipeline/internal/registry"
)

const (
	delegationThreshold = 50 // agents stop naturally via instructions
	maxSteps            = 30
)

// Agent is the universal worker — same struct for all agents.
type Agent struct {
	def      *registry.AgentDef
	llm      *llm.Client
	model    string
	maxDepth int
}

func New(def *registry.AgentDef, client *llm.Client, model string, maxDepth int) *Agent {
	return &Agent{def: def, llm: client, model: model, maxDepth: maxDepth}
}

// Run executes a task and returns the result.
// depth=0 for top-level agents, depth+1 for sub-agents.
func (a *Agent) Run(ctx context.Context, task string, depth int) (string, error) {
	pad := strings.Repeat("  ", depth)
	fmt.Printf("%s[%s] task: %s\n", pad, a.def.Name, trunc(task, 80))

	// Fresh message history — never inherits parent conversation
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(a.buildSystem()),
		openai.UserMessage(task),
	}

	stepsTaken := 0

	for range maxSteps {
		resp, err := a.llm.ChatWithTools(ctx, a.model, messages, a.def.Schemas())
		if err != nil {
			return "", fmt.Errorf("[%s] LLM error: %w", a.def.Name, err)
		}

		msg := resp.Choices[0].Message

		// Final text answer
		if len(msg.ToolCalls) == 0 {
			fmt.Printf("%s[%s] done\n", pad, a.def.Name)
			return msg.Content, nil
		}

		// Append assistant turn (SDK canonical method)
		messages = append(messages, msg.ToParam())

		// Execute tool calls
		for _, tc := range msg.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)

			fmt.Printf("%s  -> %s(%s)\n", pad, tc.Function.Name, fmtArgs(args))

			result, err := a.def.Dispatch(tc.Function.Name, args)
			if err != nil {
				result = fmt.Sprintf("error: %v", err)
			}

			fmt.Printf("%s  <- %s\n", pad, trunc(result, 100))
			stepsTaken++

			messages = append(messages, openai.ToolMessage(result, tc.ID))
		}

		// Hard stop after 8 tool calls — agent must synthesise what it has
		// (AGENT.md instructs 4 max; this is a safety net at 8)
		if stepsTaken >= 8 {
			fmt.Printf("%s[%s] hard stop at %d tool calls — forcing synthesis\n", pad, a.def.Name, stepsTaken)
			break
		}

		// Delegate only if threshold hit and depth budget remains
		if stepsTaken >= delegationThreshold && depth < a.maxDepth {
			fmt.Printf("%s[%s] delegating (%d steps)\n", pad, a.def.Name, stepsTaken)
			return a.delegate(ctx, task, messages, depth)
		}
	}

	// Hard stop fired — ask LLM to synthesise from what it has already found
	fmt.Printf("%s[%s] synthesising from %d tool calls\n", pad, a.def.Name, stepsTaken)
	messages = append(messages, openai.UserMessage(
		"You have reached the search limit. Based on everything you found so far, "+
			"give your best answer now. State any uncertainty clearly. No more tool calls.",
	))
	resp, err := a.llm.ChatWithTools(ctx, a.model, messages, nil)
	if err != nil || len(resp.Choices) == 0 {
		return fmt.Sprintf("[%s] search limit reached — could not synthesise", a.def.Name), nil
	}
	return resp.Choices[0].Message.Content, nil
}

// buildSystem assembles the system prompt from agent definition + skill instructions.
func (a *Agent) buildSystem() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are %s, a specialised AI agent.\n", a.def.Name))
	sb.WriteString(fmt.Sprintf("Role: %s\n", a.def.Role))

	// Inject SKILL.md instructions from all declared skills
	if skillInstr := a.def.SkillInstructions(); skillInstr != "" {
		sb.WriteString("\n--- SKILL INSTRUCTIONS ---\n")
		sb.WriteString(skillInstr)
		sb.WriteString("\n--- END SKILL INSTRUCTIONS ---\n")
	}

	// Inject AGENT.md body instructions
	if a.def.Body != "" {
		sb.WriteString("\n--- AGENT INSTRUCTIONS ---\n")
		sb.WriteString(a.def.Body)
		sb.WriteString("\n--- END ---\n")
	}

	sb.WriteString("\nSTOPPING RULES — hard limits, not suggestions:\n")
	sb.WriteString("1. After EVERY tool call: if you have a specific answer with a source → STOP NOW and respond.\n")
	sb.WriteString("   Do NOT search again to confirm. One credible source is enough.\n")
	sb.WriteString("2. After 4 tool calls total → STOP regardless. Report what you found.\n")
	sb.WriteString("3. Searching more does not improve accuracy — stop as soon as you have an answer.\n")
	sb.WriteString("\nWhen finished, respond with plain text only — your final answer, no tool call.")

	return sb.String()
}

// delegate decomposes the task and spawns isolated child agents.
// Each child gets a FRESH message history + a summary of parent findings.
func (a *Agent) delegate(ctx context.Context, task string, messages []openai.ChatCompletionMessageParamUnion, depth int) (string, error) {
	pad := strings.Repeat("  ", depth)

	// Summarise what the parent already found
	parentSummary := a.summariseFindings(ctx, messages, task)

	// Ask LLM to decompose into sub-tasks
	resp, err := a.llm.ChatWithTools(ctx, a.model, []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf(
			"Break this task into 2-4 independent sub-tasks.\nTask: %s\n\n"+
				"Return ONLY a JSON array of strings. No explanation, no markdown.",
			task,
		)),
	}, nil)
	if err != nil {
		return a.runFresh(ctx, task, depth)
	}

	clean := strings.TrimSpace(resp.Choices[0].Message.Content)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var subtasks []string
	if err := json.Unmarshal([]byte(clean), &subtasks); err != nil || len(subtasks) == 0 {
		fmt.Printf("%s[%s] decomposition failed — retrying solo\n", pad, a.def.Name)
		return a.runFresh(ctx, task, depth)
	}
	if len(subtasks) > 4 {
		subtasks = subtasks[:4]
	}

	fmt.Printf("%s[%s] spawning %d isolated sub-agents\n", pad, a.def.Name, len(subtasks))

	results := make([]string, 0, len(subtasks))
	for i, subtask := range subtasks {
		// Each sub-agent gets:
		//   1. Its specific sub-task
		//   2. A summary of parent findings (NOT the full message history)
		// Run() always starts with a fresh messages slice — fully isolated.
		subTask := subtask
		if parentSummary != "" {
			subTask = fmt.Sprintf(
				"Context from parent (already found):\n%s\n\nYour sub-task: %s",
				parentSummary, subtask,
			)
		}

		child := &Agent{def: a.def, llm: a.llm, model: a.model, maxDepth: a.maxDepth}
		result, _ := child.Run(ctx, subTask, depth+1)
		results = append(results, fmt.Sprintf("Sub-task %d: %s\nResult: %s", i+1, subtask, result))
	}

	// Synthesise — only final results, not child transcripts
	synthResp, err := a.llm.ChatWithTools(ctx, a.model, []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf(
			"Original task: %s\n\nSub-task results:\n%s\n\nSynthesise into one final answer.",
			task, strings.Join(results, "\n\n"),
		)),
	}, nil)
	if err != nil || len(synthResp.Choices) == 0 {
		return strings.Join(results, "\n\n"), nil
	}
	synthesis := synthResp.Choices[0].Message.Content
	if synthesis == "" {
		return strings.Join(results, "\n\n"), nil
	}
	return synthesis, nil
}

// summariseFindings distils tool results from the parent's message history
// into a compact bullet-point summary for sub-agents.
func (a *Agent) summariseFindings(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, task string) string {
	if len(messages) <= 2 {
		return ""
	}

	// Collect tool result content from message history
	var findings []string
	for _, msg := range messages {
		if msg.OfTool == nil {
			continue
		}
		// Extract content — ToolMessage content is stored as a string
		content := ""
		switch {
		case msg.OfTool.Content.OfString.Valid():
			content = msg.OfTool.Content.OfString.Value
		case len(msg.OfTool.Content.OfArrayOfContentParts) > 0:
			var parts []string
			for _, p := range msg.OfTool.Content.OfArrayOfContentParts {
				parts = append(parts, p.Text)
			}
			content = strings.Join(parts, " ")
		}
		if content == "" {
			continue
		}
		if len(content) > 600 {
			content = content[:600] + "..."
		}
		findings = append(findings, content)
	}

	if len(findings) == 0 {
		return ""
	}

	findingsText := strings.Join(findings, "\n\n---\n\n")
	if len(findingsText) > 6000 {
		findingsText = findingsText[:6000] + "\n[truncated]"
	}

	resp, err := a.llm.ChatWithTools(ctx, a.model, []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(fmt.Sprintf(
			"Summarise these findings in 3-5 bullet points. Be concise.\n\n"+
				"Task: %s\n\nFindings:\n%s",
			task, findingsText,
		)),
	}, nil)
	if err != nil || len(resp.Choices) == 0 {
		if len(findingsText) > 1000 {
			return findingsText[:1000] + "..."
		}
		return findingsText
	}
	return resp.Choices[0].Message.Content
}

func (a *Agent) runFresh(ctx context.Context, task string, depth int) (string, error) {
	fresh := &Agent{def: a.def, llm: a.llm, model: a.model, maxDepth: -1}
	return fresh.Run(ctx, task, depth)
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func fmtArgs(args map[string]any) string {
	b, _ := json.Marshal(args)
	return trunc(string(b), 60)
}
