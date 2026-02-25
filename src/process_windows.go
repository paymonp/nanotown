//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	windows.CloseHandle(handle)
	return true
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	p.Kill()
}

func forceKillProcess(pid int) {
	killProcess(pid)
}

func setupConsoleEncoding() {
	windows.SetConsoleOutputCP(65001)
	windows.SetConsoleCP(65001)
}
