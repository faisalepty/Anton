package tools

import (
	"fmt"
	"os/exec"
)

func BashTool(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error executing command: %v, output: %s", err, string(output))
	}
	return string(output), nil
}