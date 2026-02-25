package main

import (
	"fmt"
	"strings"
)

type ModelEntry struct {
	Name    string
	Command string
}

var models = []ModelEntry{
	{"claude", "claude"},
	{"opencode", "opencode"},
	{"aider", "aider"},
	{"kimi", "kimi"},
	{"codex", "codex"},
}

func getModelCommand(name string) (string, error) {
	for _, m := range models {
		if m.Name == name {
			return m.Command, nil
		}
	}
	return "", fmt.Errorf("unknown model: %s. Supported: %s", name, strings.Join(getModelNames(), ", "))
}

func isModelSupported(name string) bool {
	for _, m := range models {
		if m.Name == name {
			return true
		}
	}
	return false
}

func getModelNames() []string {
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	return names
}
