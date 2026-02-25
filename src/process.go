package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("command failed: %s\n%s", strings.Join(args, " "), result)
	}
	return result, nil
}
