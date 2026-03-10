package runtime

import (
	"agent/internal/memory"
	"agent/internal/planner"
	"agent/internal/tools"
	"agent/internal/events"
)

type Runtime struct {
	// add fields as needed
	planner *planner.Planner
	memory  *memory.Memory
	events  *events.EventHandler
}

func NewRuntime(planner *planner.Planner, memory *memory.Memory, events *events.EventHandler) *Runtime {
	return &Runtime{
		planner: planner,
		memory:  memory,
		events:  events,
	}
}

func (r *Runtime) execute_tool(tool string, args map[string]interface{}) (interface{}, error) {
	TOOLS := tools.Tools
	if toolFunc, ok := TOOLS[tool]; ok {
		// Here you would call the tool function with the provided args
		// This is a placeholder and should be replaced with actual logic to call the tool
		result := toolFunc(args)
		r.events.Emit("tool_executed", tool, result)
		return result, nil
	} else {
		return nil, fmt.Errorf("tool not found: %s", tool)
	}

}

func (r *Runtime) run(goal string) {
	// Here you would implement the logic to run the agent with the given goal
	step := 0
	for step < 10 {
		plan, err := r.planner.Plan(goal, r.memory)
		if err != nil {
			r.events.Emit("error", err)
			return
		}

		// Parse the plan and execute the tools as needed
		tool, args := r.planner.ParsePlan(plan)
		result, err := r.execute_tool(tool, args)
		if err != nil {
			r.events.Emit("error", err)
			return
		}
		r.memory.Store("Tool", result)
		fmt.Println("Step:", step)
		fmt.Println("Executed tool:", tool, "with args:", args)
		fmt.Println("Result:", result)
		if string.Contains(fmt.Sprint(result), "goal achieved") {
			r.events.Emit("goal_achieved", goal)
			fmt.Println("Goal achieved:", goal)
			return
		}
		step++
	}
}