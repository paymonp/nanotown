//go:build windows

package main

import (
	"os"
	"strings"
	"unsafe"

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

// Windows has no SIGTERM equivalent — Process.Kill() is always a hard kill
func forceKillProcess(pid int) {
	killProcess(pid)
}

func setupConsoleEncoding() {
	windows.SetConsoleOutputCP(65001) // UTF-8 codepage
	windows.SetConsoleCP(65001)
}

func getChildProcessNames(pid int) []string {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	// Collect all parent->children relationships
	type proc struct {
		pid  uint32
		ppid uint32
		name string
	}
	var procs []proc

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	err = windows.Process32First(snap, &pe)
	for err == nil {
		name := windows.UTF16ToString(pe.ExeFile[:])
		procs = append(procs, proc{pid: pe.ProcessID, ppid: pe.ParentProcessID, name: name})
		err = windows.Process32Next(snap, &pe)
	}

	// BFS to find all descendants of pid
	target := uint32(pid)
	descendants := map[uint32]bool{target: true}
	changed := true
	for changed {
		changed = false
		for _, p := range procs {
			if descendants[p.ppid] && !descendants[p.pid] {
				descendants[p.pid] = true
				changed = true
			}
		}
	}

	var names []string
	for _, p := range procs {
		if p.pid != target && descendants[p.pid] {
			names = append(names, strings.ToLower(p.name)) // Windows process names are case-insensitive
		}
	}
	return names
}
