//go:build !windows

package main

import (
	"os"
	"syscall"
)

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	p.Signal(syscall.SIGTERM)
}

func forceKillProcess(pid int) {
	if pid <= 0 {
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	p.Signal(syscall.SIGKILL)
}

func setupConsoleEncoding() {}
