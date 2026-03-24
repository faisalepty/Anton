package memory

import (
	"fmt"
	"strings"
	"sync"
)

type Entry struct {
	Agent  string
	Task   string
	Result string
}

type Memory struct {
	mu      sync.Mutex
	entries []Entry
}

func New() *Memory { return &Memory{} }

func (m *Memory) Add(agent, task, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, Entry{agent, task, result})
}

func (m *Memory) Summarize() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, e := range m.entries {
		sb.WriteString(fmt.Sprintf("Step %d [%s]: %s\n-> %s\n\n", i+1, e.Agent, e.Task, e.Result))
	}
	return sb.String()
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = nil
}