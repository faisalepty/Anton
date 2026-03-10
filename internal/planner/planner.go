package planner

import (
	"fmt"
	"agent/internal/memory"
)

type Planner struct {
	llm interface{} // replace with actual LLM type
}

func NewPlanner(llm interface{}) *Planner {
	return &Planner{
		llm: llm,
	}
}

func (p *Planner) Plan(goal string, memory *memory.Memory) (string, error) {
	context := memory.Summarize()

	prompt := fmt.Sprintf("You are an agent with the following goal: %s\n\nContext from memory: %s\n\nPlan a sequence of steps to achieve the goal. Respond in JSON {{\"tool\": \"tool_name\", \"args\":{}}}", goal, context)

	// Here you would call your LLM with the prompt and parse the response
	response := p.llm.Call(prompt)

	return response, nil
}

func (p *Planner) ParsePlan(plan string) (string, map[string]interface{}) {
	var tool string
	var args map[string]interface{}

	// Here you would implement the actual parsing logic
	// This is a placeholder implementation
	tool = "example_tool"
	args = map[string]interface{}{"arg1": "value1"}

	return tool, args
}
