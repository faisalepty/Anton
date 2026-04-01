package registry

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ToolDef struct {
	Name, SchemaPath, ScriptPath string
}

type SkillDef struct {
	Name, Instructions string
	Tools              map[string]ToolDef
}

type AgentDef struct {
	Name, Role, Instructions string
	SkillNames               []string
	ResolvedSkills           []*SkillDef
	AllowedSubAgents         []string
}

func (a *AgentDef) HasSkill(name string) bool {
	for _, s := range a.SkillNames {
		if s == name {
			return true
		}
	}
	return false
}

// Load initializes the system by crawling the agents and skills directories.
func Load(agentsDir, skillsDir string) (map[string]*AgentDef, map[string]*SkillDef, error) {
	skills := make(map[string]*SkillDef)
	agents := make(map[string]*AgentDef)

	// 1. LOAD SKILLS FIRST (Agents need them to resolve)
	sEntries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read skills dir: %w", err)
	}

	for _, e := range sEntries {
		if !e.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillsDir, e.Name())
		skill := &SkillDef{
			Name:  e.Name(),
			Tools: make(map[string]ToolDef),
		}

		// Read SKILL.md
		if data, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md")); err == nil {
			skill.Instructions = string(data)
		}

		// Scan scripts subfolder for .json schemas
		scriptsDir := filepath.Join(skillPath, "scripts")
		if files, err := os.ReadDir(scriptsDir); err == nil {
			for _, f := range files {
				if filepath.Ext(f.Name()) == ".json" {
					toolName := strings.TrimSuffix(f.Name(), ".json")
					skill.Tools[toolName] = ToolDef{
						Name:       toolName,
						SchemaPath: filepath.Join(scriptsDir, f.Name()),
						ScriptPath: findExec(scriptsDir, toolName),
					}
				}
			}
		}
		skills[e.Name()] = skill
	}

	// 2. LOAD AGENTS
	aEntries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read agents dir: %w", err)
	}

	for _, e := range aEntries {
		if !e.IsDir() {
			continue
		}

		agent := &AgentDef{Name: e.Name()}
		filePath := filepath.Join(agentsDir, e.Name(), "AGENT.md")

		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("[Registry] Warning: Could not open %s\n", filePath)
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Normalize line for header checking
			lowerLine := strings.ToLower(line)

			switch {
			case strings.HasPrefix(lowerLine, "# role:"):
				agent.Role = strings.TrimSpace(line[7:])

			case strings.HasPrefix(lowerLine, "# skills:"):
				// Remove brackets [] and colon, then split by comma
				val := strings.Trim(line[9:], " :[]")
				parts := strings.Split(val, ",")
				for _, p := range parts {
					sName := strings.TrimSpace(p)
					if sName == "" {
						continue
					}
					agent.SkillNames = append(agent.SkillNames, sName)
					if sk, ok := skills[sName]; ok {
						agent.ResolvedSkills = append(agent.ResolvedSkills, sk)
					}
				}

			case strings.HasPrefix(lowerLine, "# allowedsubagents:"):
				val := strings.Trim(line[19:], " :[]")
				parts := strings.Split(val, ",")
				for _, p := range parts {
					if sub := strings.TrimSpace(p); sub != "" {
						agent.AllowedSubAgents = append(agent.AllowedSubAgents, sub)
					}
				}

			default:
				// Everything else is treated as Instructions
				agent.Instructions += line + "\n"
			}
		}
		file.Close()
		agents[e.Name()] = agent
	}

	return agents, skills, nil
}

func findExec(dir, name string) string {
	for _, ext := range []string{".py", ".sh", ".js", ""} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
