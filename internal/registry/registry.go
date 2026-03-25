package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pipeline/internal/skill"
	"pipeline/internal/tool"
)

// AgentDef is a fully loaded agent definition.
// Tools and system prompt instructions come from the declared skills.
type AgentDef struct {
	Name        string
	Description string
	Role        string
	Body        string   // AGENT.md instructions body
	SkillNames  []string // declared in frontmatter: skills: filesystem, shell

	// Assembled from skills at load time
	tools       map[string]tool.Tool
	schemas     []tool.Schema
	skillBodies []string // SKILL.md bodies to inject into system prompt
}

// Dispatch calls the named tool from any of this agent's skills.
func (a *AgentDef) Dispatch(name string, args map[string]any) (string, error) {
	t, ok := a.tools[name]
	if !ok {
		return "", fmt.Errorf("agent '%s' has no tool '%s'", a.Name, name)
	}
	return t.Run(args)
}

// Schemas returns all tool schemas for LLM function calling.
func (a *AgentDef) Schemas() []tool.Schema { return a.schemas }

// ToolNames returns the names of all available tools.
func (a *AgentDef) ToolNames() []string {
	names := make([]string, 0, len(a.tools))
	for n := range a.tools {
		names = append(names, n)
	}
	return names
}

// SkillInstructions returns all SKILL.md bodies joined for system prompt injection.
func (a *AgentDef) SkillInstructions() string {
	if len(a.skillBodies) == 0 {
		return ""
	}
	return strings.Join(a.skillBodies, "\n\n")
}

// Registry holds all loaded agent definitions.
type Registry struct {
	agents map[string]*AgentDef
	order  []string
}

// Load scans agentsDir and loads every subfolder as an agent definition.
// skillReg provides the skill packages agents can declare.
func Load(agentsDir string, skillReg *skill.Registry) (*Registry, error) {
	reg := &Registry{agents: make(map[string]*AgentDef)}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("reading agents dir '%s': %w", agentsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentPath := filepath.Join(agentsDir, entry.Name())
		def, err := loadAgent(agentPath, entry.Name(), skillReg)
		if err != nil {
			fmt.Printf("[registry] warning: skipping '%s': %v\n", entry.Name(), err)
			continue
		}

		reg.agents[def.Name] = def
		reg.order = append(reg.order, def.Name)

		skillInfo := "no skills"
		if len(def.SkillNames) > 0 {
			skillInfo = "skills: " + strings.Join(def.SkillNames, ", ")
		}
		fmt.Printf("[registry] loaded '%s' (%s, %d tools)\n",
			def.Name, skillInfo, len(def.tools))
	}

	return reg, nil
}

func (r *Registry) Get(name string) (*AgentDef, bool) {
	def, ok := r.agents[name]
	return def, ok
}

func (r *Registry) First() *AgentDef {
	if len(r.order) == 0 {
		return nil
	}
	return r.agents[r.order[0]]
}

func (r *Registry) Names() []string { return r.order }

func (r *Registry) Menu() string {
	var sb strings.Builder
	for _, name := range r.order {
		def := r.agents[name]
		desc := def.Description
		if len(desc) > 120 {
			desc = desc[:120] + "..."
		}
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", name, desc))
	}
	return sb.String()
}

// ── agent loader ──────────────────────────────────────────────────────────────

func loadAgent(agentPath, folderName string, skillReg *skill.Registry) (*AgentDef, error) {
	agentFile := filepath.Join(agentPath, "AGENT.md")
	data, err := os.ReadFile(agentFile)
	if err != nil {
		return nil, fmt.Errorf("AGENT.md not found")
	}

	name, description, role, skillNames, body, err := parseAgentMD(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse AGENT.md: %w", err)
	}
	if name == "" {
		name = folderName
	}

	// Wire up tools and instructions from declared skills
	toolMap := make(map[string]tool.Tool)
	var schemas []tool.Schema
	var skillBodies []string

	for _, skillName := range skillNames {
		s, ok := skillReg.Get(skillName)
		if !ok {
			fmt.Printf("[registry] warning: agent '%s' declared unknown skill '%s'\n", name, skillName)
			continue
		}
		// Merge tools from this skill
		for _, schema := range s.Schemas() {
			schemas = append(schemas, schema)
		}
		// Register tool dispatch — route by tool name through the skill
		skillCopy := s // capture for closure
		for _, toolName := range s.ToolNames() {
			tn := toolName
			sk := skillCopy
			toolMap[tn] = &skillToolAdapter{skillName: sk.Name, toolName: tn, skill: sk}
		}
		// Collect SKILL.md instructions for system prompt
		if s.Body != "" {
			skillBodies = append(skillBodies, fmt.Sprintf("=== %s skill ===\n%s", skillName, s.Body))
		}
	}

	return &AgentDef{
		Name:        name,
		Description: description,
		Role:        role,
		Body:        body,
		SkillNames:  skillNames,
		tools:       toolMap,
		schemas:     schemas,
		skillBodies: skillBodies,
	}, nil
}

// skillToolAdapter bridges AgentDef.Dispatch to the correct Skill.
type skillToolAdapter struct {
	skillName string
	toolName  string
	skill     *skill.Skill
}

func (a *skillToolAdapter) Schema() tool.Schema {
	for _, s := range a.skill.Schemas() {
		if s.Function.Name == a.toolName {
			return s
		}
	}
	return tool.Schema{}
}

func (a *skillToolAdapter) Run(args map[string]any) (string, error) {
	return a.skill.Dispatch(a.toolName, args)
}

// ── AGENT.md parser ───────────────────────────────────────────────────────────

func parseAgentMD(content string) (name, description, role string, skills []string, body string, err error) {
	re := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)`)
	matches := re.FindStringSubmatch(content)
	if matches == nil {
		return "", "", "", nil, "", fmt.Errorf("missing frontmatter")
	}
	fm := matches[1]
	body = strings.TrimSpace(matches[2])

	name = fmField(fm, "name")
	description = fmField(fm, "description")
	role = fmField(fm, "role")

	// Parse skills: comma-separated string
	skillsRaw := fmField(fm, "skills")
	if skillsRaw != "" {
		for _, s := range strings.Split(skillsRaw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				skills = append(skills, s)
			}
		}
	}
	return
}

func fmField(fm, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.+?)(?:\n\w|\z)`)
	m := re.FindStringSubmatch(fm)
	if m == nil {
		return ""
	}
	val := strings.TrimSpace(m[1])
	val = strings.Trim(val, `"`)
	val = regexp.MustCompile(`\n\s+`).ReplaceAllString(val, " ")
	return val
}
