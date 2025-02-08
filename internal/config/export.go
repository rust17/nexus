package config

import (
	"os"
	"testing"
)

var CreateTempConfigFile = createTempConfigFile

func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	return tmpFile.Name()
}
