//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func getChildProcessNames(pid int) []string {
	// Try /proc first (Linux)
	if names := getChildProcessNamesProc(pid); names != nil {
		return names
	}
	// Fallback: use ps (macOS, other Unix)
	return getChildProcessNamesPS(pid)
}

func getChildProcessNamesProc(pid int) []string {
	// Check if /proc exists
	if _, err := os.Stat("/proc"); err != nil {
		return nil
	}

	// Read all /proc/<pid>/stat to build parent->child map
	type proc struct {
		pid  int
		ppid int
		name string
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var procs []proc
	for _, entry := range entries {
		childPid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		// Format: <pid> (<name>) <state> <ppid> ...
		fields := string(data)
		// Name is between first '(' and last ')'
		nameStart := strings.IndexByte(fields, '(')
		nameEnd := strings.LastIndexByte(fields, ')')
		if nameStart < 0 || nameEnd < 0 || nameEnd <= nameStart {
			continue
		}
		name := fields[nameStart+1 : nameEnd]
		rest := strings.Fields(fields[nameEnd+2:])
		if len(rest) < 2 {
			continue
		}
		ppid, err := strconv.Atoi(rest[1])
		if err != nil {
			continue
		}
		procs = append(procs, proc{pid: childPid, ppid: ppid, name: name})
	}

	// BFS to find all descendants
	descendants := map[int]bool{pid: true}
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
		if p.pid != pid && descendants[p.pid] {
			names = append(names, p.name)
		}
	}
	return names
}

func getChildProcessNamesPS(pid int) []string {
	// Use ps to list all processes with pid, ppid, comm
	output, err := runCommand(".", "ps", "-eo", "pid=,ppid=,comm=")
	if err != nil {
		return nil
	}

	type proc struct {
		pid  int
		ppid int
		name string
	}
	var procs []proc
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		p, err1 := strconv.Atoi(fields[0])
		pp, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		name := filepath.Base(fields[2])
		procs = append(procs, proc{pid: p, ppid: pp, name: name})
	}

	// BFS to find all descendants
	descendants := map[int]bool{pid: true}
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
		if p.pid != pid && descendants[p.pid] {
			names = append(names, p.name)
		}
	}
	return names
}
