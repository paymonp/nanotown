package main

var knownAgents = []string{
	"claude", "opencode", "aider", "kimi", "codex",
	"gemini", "copilot", "qwen",
}

func detectModel(pid int) string {
	if pid <= 0 {
		return ""
	}
	children := getChildProcessNames(pid)
	for _, name := range children {
		for _, agent := range knownAgents {
			if name == agent || name == agent+".exe" {
				return agent
			}
		}
	}
	return ""
}
