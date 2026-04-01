package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Task struct {
	ID        string   `json:"id"`
	Agent     string   `json:"agent"`
	Input     string   `json:"input"`
	DependsOn []string `json:"depends_on"`
}

type Plan struct {
	Tasks []Task `json:"tasks"`
}

func (b *Brain) ExecutePlan(ctx context.Context, plan Plan) (map[string]string, error) {
	results := make(map[string]string)
	channels := make(map[string]chan bool)
	for _, t := range plan.Tasks {
		channels[t.ID] = make(chan bool)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	indent := strings.Repeat("  ", b.depth)

	fmt.Printf("%s[Coordinator] Executing DAG with %d tasks...\n", indent, len(plan.Tasks))

	for _, t := range plan.Tasks {
		wg.Add(1)
		go func(task Task) {
			defer wg.Done()

			// Wait for dependencies
			if len(task.DependsOn) > 0 {
				fmt.Printf("%s  [Task: %s] Waiting for dependencies: %v\n", indent, task.ID, task.DependsOn)
				for _, depID := range task.DependsOn {
					<-channels[depID]
				}
			}

			fmt.Printf("%s  [Task: %s] Launching Agent: %s\n", indent, task.ID, task.Agent)

			// Use depth+1 for the sub-brain
			subBrain := NewBrain(b.client, b.model, b.agents, b.depth+1)
			res, err := subBrain.Execute(ctx, task.Agent, task.Input)

			mu.Lock()
			if err != nil {
				results[task.ID] = fmt.Sprintf("Failed: %v", err)
			} else {
				results[task.ID] = res
			}
			mu.Unlock()

			fmt.Printf("%s  [Task: %s] Completed.\n", indent, task.ID)
			close(channels[task.ID])
		}(t)
	}
	wg.Wait()
	fmt.Printf("%s[Coordinator] All tasks in plan finished.\n", indent)
	return results, nil
}
