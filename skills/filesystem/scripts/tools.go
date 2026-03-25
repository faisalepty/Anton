package filesystemscripts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pipeline/internal/tool"
)

// Tools returns all filesystem tools.
func Tools() []tool.Tool {
	return []tool.Tool{ReadFile{}, WriteFile{}, AppendFile{}, ListDirectory{}}
}

// ── ReadFile ──────────────────────────────────────────────────────────────────

type ReadFile struct{}

func (r ReadFile) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name:        "read_file",
		Description: "Read the contents of a file.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []string{"path"},
		},
	}}
}

func (r ReadFile) Run(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing 'path'")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("not found: %s", abs)
	}
	if info.IsDir() {
		return "", fmt.Errorf("'%s' is a directory", abs)
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(content)
	if len(text) > 20000 {
		text = text[:20000] + fmt.Sprintf("\n[truncated — %d bytes total]", info.Size())
	}
	return text, nil
}

// ── WriteFile ─────────────────────────────────────────────────────────────────

type WriteFile struct{}

func (w WriteFile) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name:        "write_file",
		Description: "Write content to a file. Creates or overwrites. Use patch_file for partial edits.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
	}}
}

func (w WriteFile) Run(args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", fmt.Errorf("missing 'path'")
	}
	abs, _ := filepath.Abs(path)
	os.MkdirAll(filepath.Dir(abs), 0755)
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Written %d chars to %s", len(content), abs), nil
}

// ── AppendFile ────────────────────────────────────────────────────────────────

type AppendFile struct{}

func (a AppendFile) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name:        "append_file",
		Description: "Append content to an existing file without overwriting.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
	}}
}

func (a AppendFile) Run(args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", fmt.Errorf("missing 'path'")
	}
	abs, _ := filepath.Abs(path)
	os.MkdirAll(filepath.Dir(abs), 0755)
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	f.WriteString(content)
	return fmt.Sprintf("Appended %d chars to %s", len(content), abs), nil
}

// ── ListDirectory ─────────────────────────────────────────────────────────────

type ListDirectory struct{}

func (l ListDirectory) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name:        "list_directory",
		Description: "List files and folders in a directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "default": "."},
			},
			"required": []string{},
		},
	}}
}

func (l ListDirectory) Run(args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	abs, _ := filepath.Abs(path)
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	var lines []string
	lines = append(lines, abs+"/")
	for _, e := range entries {
		if e.IsDir() {
			lines = append(lines, fmt.Sprintf("  [dir]  %s/", e.Name()))
		} else {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			lines = append(lines, fmt.Sprintf("  [file] %s (%d bytes)", e.Name(), size))
		}
	}
	return strings.Join(lines, "\n"), nil
}