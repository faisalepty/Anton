package tools

import (
	"fmt"
	"os"
)

func WriteFile(filePath string, content string) error {
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}
	return nil
}
