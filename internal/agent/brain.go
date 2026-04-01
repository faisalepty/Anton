package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"pipeline/internal/llm"
	"pipeline/internal/registry"
	"pipeline/internal/runner"
	"pipeline/internal/tool"

	"github.com/openai/openai-go/v3"
)

type Brain struct {
	client *llm.Client
	model  string
	agents map[string]*registry.AgentDef
	run    *runner.Runner
	depth  int
}

func NewBrain(client *llm.Client, model string, agents map[string]*registry.AgentDef, depth int) *Brain {
	return &Brain{client: client, model: model, agents: agents, run: runner.New(), depth: depth}
}

func (b *Brain) Execute(ctx context.Context, agentName, input string) (string, error) {
	if b.depth > 3 {
		return "Error: Max recursion depth reached", nil
	}

	indent := strings.Repeat("  ", b.depth)
	fmt.Printf("%s[Agent: %s] Starting task: %s\n", indent, agentName, input)

	def, ok := b.agents[agentName]
	if !ok {
		return "", fmt.Errorf("agent %s not found", agentName)
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(b.buildPrompt(def)),
		openai.UserMessage(input),
	}

	schemas := b.getTools(def)

	for i := 0; i < 10; i++ {
		fmt.Printf("%s[Agent: %s] Thinking (Turn %d)...\n", indent, agentName, i+1)

		resp, err := b.client.ChatWithTools(ctx, b.model, messages, schemas)
		if err != nil {
			return "", err
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg.ToParam())

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("%s[Agent: %s] Final Answer generated.\n", indent, agentName)
			return msg.Content, nil
		}

		for _, tc := range msg.ToolCalls {
			fmt.Printf("%s[Action] %s calling tool: %s\n", indent, agentName, tc.Function.Name)

			var res string
			if tc.Function.Name == "submit_plan" {
				var p Plan
				json.Unmarshal([]byte(tc.Function.Arguments), &p)
				fmt.Printf("%s[Plan] Manager submitted a plan with %d tasks.\n", indent, len(p.Tasks))

				results, _ := b.ExecutePlan(ctx, p)
				resB, _ := json.Marshal(results)
				res = string(resB)
			} else {
				tDef, found := b.findTool(def, tc.Function.Name)
				if !found {
					res = "Error: Tool not found"
				} else {
					res, err = b.run.Run(ctx, tDef.ScriptPath, tc.Function.Arguments)
					if err != nil {
						fmt.Printf("%s[Error] Tool %s failed: %v\n", indent, tc.Function.Name, err)
						res = fmt.Sprintf("Error: %v", err)
					}
				}
			}
			fmt.Printf("%s[Observation] Tool %s returned data.\n", indent, tc.Function.Name)
			messages = append(messages, openai.ToolMessage(tc.ID, res))
		}
	}
	return "Loop limit reached", nil
}

func (b *Brain) getTools(d *registry.AgentDef) []tool.Schema {
	var schemas []tool.Schema
	for _, skill := range d.ResolvedSkills {
		for _, t := range skill.Tools {
			data, _ := os.ReadFile(t.SchemaPath)
			var f tool.FunctionSchema
			if err := json.Unmarshal(data, &f); err == nil {
				schemas = append(schemas, tool.Schema{Type: "function", Function: f})
			}
		}
	}
	if d.HasSkill("delegation") {
		schemas = append(schemas, tool.Schema{
			Type: "function",
			Function: tool.FunctionSchema{
				Name:        "submit_plan",
				Description: "Run multiple sub-agents in parallel based on dependencies.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tasks": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":         map[string]any{"type": "string"},
									"agent":      map[string]any{"type": "string", "enum": d.AllowedSubAgents},
									"input":      map[string]any{"type": "string"},
									"depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
								},
							},
						},
					},
				},
			},
		})
	}
	return schemas
}

func (b *Brain) buildPrompt(d *registry.AgentDef) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Role: %s\n\nInstructions:\n%s\n", d.Role, d.Instructions))
	for _, s := range d.ResolvedSkills {
		sb.WriteString(fmt.Sprintf("\nSkill [%s] rules:\n%s\n", s.Name, s.Instructions))
	}
	return sb.String()
}

func (b *Brain) findTool(d *registry.AgentDef, name string) (registry.ToolDef, bool) {
	for _, s := range d.ResolvedSkills {
		if t, ok := s.Tools[name]; ok {
			return t, true
		}
	}
	return registry.ToolDef{}, false
}
