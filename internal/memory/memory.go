package memory

import (
	"fmt"
	"strings"
	"sync"
)

// Entry is one completed step stored in memory.
type Entry struct {
	Agent  string
	Task   string
	Result string
}

// Memory stores completed step results across a conversation.
// The planner uses Summarize() to pass prior context to each new plan.
type Memory struct {
	mu      sync.Mutex
	entries []Entry
}

// New creates an empty Memory.
func New() *Memory {
	return &Memory{}
}

// Add records a completed agent step.
func (m *Memory) Add(agent, task, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, Entry{
		Agent:  agent,
		Task:   task,
		Result: result,
	})
}

// Summarize returns all stored entries as a plain-text block.
// Returned to the planner as prior context on the next prompt.
func (m *Memory) Summarize() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, e := range m.entries {
		sb.WriteString(fmt.Sprintf(
			"Step %d [%s]: %s\n→ %s\n\n",
			i+1, e.Agent, e.Task, e.Result,
		))
	}
	return sb.String()
}

// Clear resets the memory store.
func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = nil
}