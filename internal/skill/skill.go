package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"pipeline/internal/tool"
)

// Skill is a loaded skill package.
// It provides both instructions (Body) and executable tools (Tools/Schemas).
type Skill struct {
	Name    string
	Body    string // SKILL.md body — injected into agent system prompt
	tools   map[string]tool.Tool
	schemas []tool.Schema
}


type SkillFM struct {
	Name string `yaml:"name"`
}

// Dispatch calls a tool by name with the given args.
func (s *Skill) Dispatch(name string, args map[string]any) (string, error) {
	t, ok := s.tools[name]
	if !ok {
		return "", fmt.Errorf("skill '%s' has no tool '%s'", s.Name, name)
	}
	return t.Run(args)
}

// Schemas returns all tool schemas for LLM function calling.
func (s *Skill) Schemas() []tool.Schema { return s.schemas }

// ToolNames returns names of all tools in this skill.
func (s *Skill) ToolNames() []string {
	names := make([]string, 0, len(s.tools))
	for n := range s.tools {
		names = append(names, n)
	}
	return names
}

// Registry loads and indexes all skills from the skills/ directory.
type Registry struct {
	skills map[string]*Skill
}

// Load scans the skills directory and loads every subfolder as a skill.
// toolProviders maps skill name → function that returns that skill's tools.
// This is how skills register their scripts without dynamic loading.
func Load(skillsDir string, toolProviders map[string]func() []tool.Tool) (*Registry, error) {
	reg := &Registry{skills: make(map[string]*Skill)}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil // skills dir not present is fine
		}
		return nil, fmt.Errorf("reading skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillsDir, entry.Name())
		skillFile := filepath.Join(skillPath, "SKILL.md")

		data, err := os.ReadFile(skillFile)
		if err != nil {
			fmt.Printf("[skill] warning: skipping '%s' (no SKILL.md)\n", entry.Name())
			continue
		}

		name, body := parseSkillMD(string(data), entry.Name())

		// Build tool map from registered providers
		toolMap := make(map[string]tool.Tool)
		var schemas []tool.Schema

		if providerFn, ok := toolProviders[name]; ok {
			for _, t := range providerFn() {
				s := t.Schema()
				toolMap[s.Function.Name] = t
				schemas = append(schemas, s)
			}
		}

		reg.skills[name] = &Skill{
			Name:    name,
			Body:    body,
			tools:   toolMap,
			schemas: schemas,
		}

		toolNames := make([]string, 0, len(toolMap))
		for n := range toolMap {
			toolNames = append(toolNames, n)
		}
		fmt.Printf("[skill] loaded '%s' (%d tools: %s)\n",
			name, len(toolMap), strings.Join(toolNames, ", "))
	}

	return reg, nil
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	s, ok := r.skills[name]
	return s, ok
}



func parseSkillMD(content, folderName string) (name, body string) {
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n?(.*)`)
	matches := re.FindStringSubmatch(content)

	// No frontmatter → fallback
	if matches == nil {
		return folderName, strings.TrimSpace(content)
	}

	fmRaw := matches[1]
	body = strings.TrimSpace(matches[2])

	var fm SkillFM
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		// fallback if YAML is broken
		return folderName, body
	}

	if fm.Name != "" {
		return fm.Name, body
	}

	return folderName, body
}