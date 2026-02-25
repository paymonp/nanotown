package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerFrame() string {
	return spinnerFrames[int(time.Now().UnixMilli()/80)%len(spinnerFrames)]
}

func sortSessions(sessions []*Session) {
	sort.SliceStable(sessions, func(i, j int) bool {
		oi, oj := statusOrder(sessions[i]), statusOrder(sessions[j])
		if oi != oj {
			return oi < oj
		}
		// Secondary sort: most recently active first
		return lastActiveTime(sessions[i]).After(lastActiveTime(sessions[j]))
	})
}

func statusOrder(s *Session) int {
	if s.Alive && isProcessAlive(s.PID) {
		t, err := time.Parse(time.RFC3339Nano, s.LastOutputAt)
		if err != nil || time.Since(t).Seconds() < 2 {
			return 0 // active
		}
		return 1 // idle
	}
	return 2 // exited
}

func lastActiveTime(s *Session) time.Time {
	if s.LastOutputAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, s.LastOutputAt); err == nil {
			return t
		}
	}
	if s.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, s.StartedAt); err == nil {
			return t
		}
	}
	return time.Time{}
}

// renderStatus builds the status tables as a string and returns the line count.
func renderStatus(sessions []*Session, sm *SessionManager, repoPath string) (string, int) {
	updateAliveFlags(sessions, sm)
	sortSessions(sessions)

	var b strings.Builder
	lines := 0

	// Build worktree list first (used by both tables)
	worktrees := listWorktrees(repoPath, sessions)
	wtBranch := map[string]string{}
	for _, wt := range worktrees {
		wtBranch[wt.id] = wt.sourceBranch
	}

	// Sessions table
	if len(sessions) == 0 {
		b.WriteString("No active sessions.")
		lines++
	} else {
		fmt.Fprintf(&b, "%-7s %-10s %-12s %-10s %-10s %-16s %-10s %-12s %s",
			"SESSION", "MODEL", "STATUS", "BRANCH", "WORKTREE", "REPO", "STARTED", "LAST ACTIVE", "DESCRIPTION")
		lines++

		for _, s := range sessions {
			status := formatSessionStatus(s, true)
			started := formatTimeAgo(s.StartedAt)
			wt := s.Worktree
			if wt == "" {
				wt = s.ID
			}
			branch := wtBranch[wt]
			if branch == "" {
				branch = "?"
			}
			fmt.Fprintf(&b, "\n%-7s %-10s %s %-10s %-10s %-16s %-10s %-12s %s",
				s.ID, s.Model, status, branch, wt, shortRepoPath(s.RepoPath), started, formatTimeAgo(s.LastOutputAt), s.Description)
			lines++
		}
	}

	// Worktree table
	if len(worktrees) > 0 {
		fmt.Fprintf(&b, "\n")
		lines++
		fmt.Fprintf(&b, "\n%-10s %-10s %s", "BRANCH", "WORKTREE", "SESSIONS")
		lines++
		for _, wt := range worktrees {
			branch := wt.sourceBranch
			if branch == "" {
				branch = "?"
			}
			fmt.Fprintf(&b, "\n%-10s %-10s %s", branch, wt.id, wt.sessionList)
			lines++
		}
	}

	fmt.Fprintf(&b, "\n")
	lines++
	fmt.Fprintf(&b, "\n\033[2mCtrl+C to exit\033[0m")
	lines++

	return b.String(), lines
}

type worktreeInfo struct {
	id           string
	sourceBranch string
	sessionList  string
}

// listWorktrees scans .nanotown/ for worktree directories and maps sessions to each.
func listWorktrees(repoPath string, sessions []*Session) []worktreeInfo {
	if repoPath == "" {
		return nil
	}
	ntDir := filepath.Join(repoPath, ".nanotown")
	entries, err := os.ReadDir(ntDir)
	if err != nil {
		return nil
	}

	// Build map of worktree ID -> session IDs
	wtSessions := map[string][]string{}
	for _, s := range sessions {
		wt := s.Worktree
		if wt == "" {
			wt = s.ID
		}
		wtSessions[wt] = append(wtSessions[wt], s.ID)
	}

	var result []worktreeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		sessionIDs := wtSessions[name]
		label := "(none)"
		if len(sessionIDs) > 0 {
			label = strings.Join(sessionIDs, ", ")
		}
		source := ""
		data, err := os.ReadFile(filepath.Join(ntDir, name, ".nt-source-branch"))
		if err == nil {
			source = strings.TrimSpace(string(data))
		}
		result = append(result, worktreeInfo{id: name, sourceBranch: source, sessionList: label})
	}
	return result
}

func updateAliveFlags(sessions []*Session, sm *SessionManager) {
	for _, s := range sessions {
		if s.Alive && !isProcessAlive(s.PID) {
			s.Alive = false
			sm.Write(s) // best-effort
		}
	}
}

func shortRepoPath(fullPath string) string {
	fullPath = strings.ReplaceAll(fullPath, "\\", "/")
	parts := strings.Split(strings.TrimRight(fullPath, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return fullPath
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func formatSessionStatus(s *Session, showSpinner bool) string {
	if !s.Alive {
		return fmt.Sprintf("\033[2m%-12s\033[0m", "exited") // dim
	}
	prefix := ""
	if showSpinner {
		prefix = spinnerFrame() + " "
	}
	if s.LastOutputAt == "" {
		return fmt.Sprintf("\033[32m%-12s\033[0m", prefix+"active") // green
	}
	t, err := time.Parse(time.RFC3339Nano, s.LastOutputAt)
	if err != nil {
		return fmt.Sprintf("\033[32m%-12s\033[0m", prefix+"active")
	}
	seconds := int(time.Since(t).Seconds())
	if seconds < 2 {
		return fmt.Sprintf("\033[32m%-12s\033[0m", prefix+"active") // green
	}
	label := prefix + "idle " + formatDuration(seconds)
	return fmt.Sprintf("\033[33m%-12s\033[0m", label) // yellow
}

func formatTimeAgo(isoTimestamp string) string {
	if isoTimestamp == "" {
		return "?"
	}
	t, err := time.Parse(time.RFC3339Nano, isoTimestamp)
	if err != nil {
		return "?"
	}
	seconds := int(time.Since(t).Seconds())
	return formatDuration(seconds) + " ago"
}

func formatDuration(totalSeconds int) string {
	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	}
	if totalSeconds < 3600 {
		return fmt.Sprintf("%dm", totalSeconds/60)
	}
	return fmt.Sprintf("%dh", totalSeconds/3600)
}
