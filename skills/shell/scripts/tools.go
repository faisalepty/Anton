package shellscripts

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"pipeline/internal/tool"
)

func Tools() []tool.Tool {
	return []tool.Tool{RunCommand{}}
}

type RunCommand struct{}

func (r RunCommand) Schema() tool.Schema {
	return tool.Schema{Type: "function", Function: tool.FunctionSchema{
		Name: "run_command",
		Description: "Execute a shell command and return stdout + stderr combined. " +
			"Use to compile, run tests, or execute scripts. Returns exit code and full output. " +
			"30 second timeout. Do not use for destructive operations.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Command to run, e.g. 'go build ./...' or 'go test ./...'",
				},
				"working_dir": map[string]any{
					"type":        "string",
					"description": "Directory to run the command from. Defaults to current directory.",
				},
			},
			"required": []string{"command"},
		},
	}}
}

var blocked = []string{"rm -rf /", "rm -rf ~", "mkfs", "DROP DATABASE", "DROP TABLE", ":(){:|:&};:"}

func (r RunCommand) Run(args map[string]any) (string, error) {
	command, _ := args["command"].(string)
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("missing 'command'")
	}

	lower := strings.ToLower(command)
	for _, b := range blocked {
		if strings.Contains(lower, strings.ToLower(b)) {
			return "", fmt.Errorf("blocked: command contains dangerous pattern '%s'", b)
		}
	}

	workingDir, _ := args["working_dir"].(string)
	if workingDir != "" {
		workingDir, _ = filepath.Abs(workingDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "exit code: -1\noutput:\n[command timed out after 30s]", nil
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Sprintf("exit code: -1\noutput:\n%s", runErr.Error()), nil
		}
	}

	output := out.String()
	if output == "" {
		output = "(no output)"
	}
	const maxOutput = 8000
	truncated := ""
	if len(output) > maxOutput {
		output = output[:maxOutput]
		truncated = fmt.Sprintf("\n[output truncated at %d chars]", maxOutput)
	}

	return fmt.Sprintf("exit code: %d\noutput:\n%s%s", exitCode, output, truncated), nil
}