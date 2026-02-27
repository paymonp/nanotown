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

// Derives frame from wall clock so all spinners animate in sync
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
			return 0 // active — 2s threshold prevents flickering during normal agent pauses
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

// refreshSessionInfo runs expensive operations (git, process scanning) and caches results on sessions.
func refreshSessionInfo(sessions []*Session, sm *SessionManager, worktrees []worktreeInfo) {
	updateAliveFlags(sessions, sm)

	// Detect models for alive sessions
	for _, s := range sessions {
		if s.Alive && isProcessAlive(s.PID) {
			detected := detectModel(s.PID)
			if detected != "" {
				s.Model = detected
				sm.Write(s) // cache for after exit
			}
		}
	}

	// Cache source branch on each session from worktree info
	wtBranch := map[string]string{}
	for _, wt := range worktrees {
		wtBranch[wt.id] = wt.branch
	}
	for _, s := range sessions {
		wt := resolveWorktreeID(s)
		branch := wtBranch[wt]
		if branch == "" && s.WorkingCopyPath != "" {
			branch = readSourceBranch(s.WorkingCopyPath)
		}
		s.SourceBranch = branch
	}
}

// renderStatus builds the status tables as a string and returns the line count.
// Uses pre-computed worktrees and cached session info (no expensive operations).
func renderStatus(sessions []*Session, worktrees []worktreeInfo) (string, int) {
	sortSessions(sessions)

	var b strings.Builder
	lines := 0

	wtDesc := map[string]string{}
	for _, wt := range worktrees {
		wtDesc[wt.id] = wt.description
	}

	// Sessions table
	b.WriteString("Sessions")
	lines++
	if len(sessions) == 0 {
		b.WriteString("\n  No active sessions.")
		lines++
	} else {
		fmt.Fprintf(&b, "\n%-16s %-10s %-7s %-10s %-12s %-10s %-10s %-12s %s",
			"REPO", "BRANCH", "SESSION", "MODEL", "STATUS", "WORKTREE", "STARTED", "LAST ACTIVE", "DESCRIPTION")
		lines++

		for _, s := range sessions {
			model := s.Model
			if model == "" {
				model = "\u2014"
			}
			status := formatSessionStatus(s, true)
			started := formatTimeAgo(s.StartedAt)
			wt := resolveWorktreeID(s)
			branch := s.SourceBranch
			if branch == "" {
				branch = "?"
			}
			fmt.Fprintf(&b, "\n%-16s %-10s %-7s %-10s %s %-10s %-10s %-12s %s",
				shortRepoPath(s.RepoPath), branch, s.ID, model, status, wt, started, formatTimeAgo(s.LastOutputAt), wtDesc[wt])
			lines++
		}
	}

	// Worktree table
	if len(worktrees) > 0 {
		fmt.Fprintf(&b, "\n")
		lines++
		fmt.Fprintf(&b, "\nWorktrees")
		lines++
		fmt.Fprintf(&b, "\n%-16s %-10s %-10s %-10s %s", "REPO", "BRANCH", "WORKTREE", "SESSIONS", "DESCRIPTION")
		lines++
		for _, wt := range worktrees {
			branch := wt.branch
			if branch == "" {
				branch = "?"
			}
			fmt.Fprintf(&b, "\n%-16s %-10s %-10s %-10s %s", shortRepoPath(wt.repo), branch, wt.id, wt.sessionList, wt.description)
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
	id          string
	repo        string
	branch      string
	description string
	sessionList string
}

// readSourceBranch reads the source branch from the .nt-source-branch file in a worktree directory.
func readSourceBranch(wtPath string) string {
	data, err := os.ReadFile(filepath.Join(wtPath, ".nt-source-branch"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// resolveWorktreeID returns the worktree ID for a session.
// Priority: WorkingCopyPath (canonical), then explicit Worktree field, then session ID.
func resolveWorktreeID(s *Session) string {
	if s.WorkingCopyPath != "" {
		return filepath.Base(s.WorkingCopyPath)
	}
	if s.Worktree != "" {
		return s.Worktree
	}
	return s.ID
}

// listWorktrees scans .nanotown/ directories across all repos referenced by sessions.
func listWorktrees(sessions []*Session) []worktreeInfo {
	// Collect unique repo paths from sessions
	repoPaths := map[string]bool{}
	for _, s := range sessions {
		if s.RepoPath != "" {
			repoPaths[s.RepoPath] = true
		}
	}

	// Build map of worktree ID -> session IDs
	wtSessions := map[string][]string{}
	for _, s := range sessions {
		wt := resolveWorktreeID(s)
		wtSessions[wt] = append(wtSessions[wt], s.ID)
	}

	var result []worktreeInfo
	for repoPath := range repoPaths {
		ntDir := filepath.Join(repoPath, ".nanotown")
		entries, err := os.ReadDir(ntDir)
		if err != nil {
			continue
		}
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
			wtPath := filepath.Join(ntDir, name)
			branch := readSourceBranch(wtPath)
			desc := ""
			data, err := os.ReadFile(filepath.Join(wtPath, ".nt-description"))
			if err == nil {
				desc = strings.TrimSpace(string(data))
			}
			result = append(result, worktreeInfo{id: name, repo: repoPath, branch: branch, description: desc, sessionList: label})
		}
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
