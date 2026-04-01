package runner

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
)

type Runner struct {
	Interpreters map[string]string
}

func New() *Runner {
	return &Runner{Interpreters: map[string]string{".py": "python3", ".js": "node", ".sh": "bash"}}
}

func (r *Runner) Run(ctx context.Context, path, args string) (string, error) {
	ext := filepath.Ext(path)
	interp, ok := r.Interpreters[ext]
	var cmd *exec.Cmd
	if ok {
		cmd = exec.CommandContext(ctx, interp, path, args)
	} else {
		cmd = exec.CommandContext(ctx, path, args)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}
