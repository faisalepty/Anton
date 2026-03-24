package tool

// Tool is the interface every script in a skill's scripts/ folder must implement.
// Schema() returns the OpenAI function-calling schema.
// Run() executes the tool and returns a string result.
type Tool interface {
	Schema() Schema
	Run(args map[string]any) (string, error)
}

// Schema is the top-level OpenAI function tool definition.
type Schema struct {
	Type     string         `json:"type"`
	Function FunctionSchema `json:"function"`
}

// FunctionSchema describes the callable to the LLM.
type FunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}