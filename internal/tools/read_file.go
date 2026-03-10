package tools

import (
	"fmt"
	"os"
)

func ReadFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}
	return string(data), nil
}
