package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	setupConsoleEncoding()
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	sm, err := NewSessionManager()
	if err != nil {
		return err
	}
	cwd, err2 := os.Getwd()
	if err2 != nil {
		return fmt.Errorf("failed to get working directory: %w", err2)
	}

	if len(args) == 0 {
		printUsage()
		return nil
	}

	command := args[0]
	switch command {
	case "status":
		return cmdLiveStatus(sm)
	case "stop":
		if len(args) < 2 {
			return fmt.Errorf("usage: nt stop <worktree-id>")
		}
		return cmdStop(args[1], sm, cwd)
	case "stopall":
		return cmdStopAll(sm, cwd)
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: nt delete <worktree-id>")
		}
		return cmdRm(args[1], sm, cwd)
	case "merge":
		if len(args) < 2 {
			return fmt.Errorf("usage: nt merge <worktree-id>")
		}
		return cmdMerge(args[1], sm, cwd)
	case "clean":
		return cmdAutoClean(sm)
	case "deleteall":
		return cmdDeleteAll(sm, cwd)
	case "help":
		printUsage()
		return nil
	default:
		// Treat all non-command args as session creation: nt <desc> or nt -w <id> [desc]
		worktreeID, desc := parseSessionArgs(args)
		if worktreeID == "" && desc == "" {
			return fmt.Errorf("description is required. Usage: nt <desc>")
		}
		return startSession(sm, cwd, desc, worktreeID)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: nt <command>")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Lifecycle:")
	fmt.Fprintln(os.Stderr, "  nt <desc>                     Launch a session")
	fmt.Fprintln(os.Stderr, "  nt -w <worktree-id> [desc]    Launch a session with a custom worktree ID")
	fmt.Fprintln(os.Stderr, "  nt status                     Show all sessions (live-updating)")
	fmt.Fprintln(os.Stderr, "  nt merge <worktree-id>        Merge into your current VCS branch and clean up")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Cleanup:")
	fmt.Fprintln(os.Stderr, "  nt stop <worktree-id>         Stop all running sessions on a worktree")
	fmt.Fprintln(os.Stderr, "  nt stopall                    Stop all running sessions")
	fmt.Fprintln(os.Stderr, "  nt clean                      Remove stopped sessions and orphaned worktrees")
	fmt.Fprintln(os.Stderr, "  nt delete <worktree-id>       Delete a worktree and its sessions")
	fmt.Fprintln(os.Stderr, "  nt deleteall                  Delete all sessions and worktrees")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Info:")
	fmt.Fprintln(os.Stderr, "  nt help                       Show this help message")
}


func parseSessionArgs(args []string) (worktreeID string, desc string) {
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-w" && i+1 < len(args) {
			worktreeID = args[i+1]
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	desc = strings.Join(rest, " ")
	return
}

func startSession(sm *SessionManager, cwd string, desc string, worktreeID string) error {
	vcs := detectVcs(cwd)
	if vcs == nil {
		return fmt.Errorf("not inside a version-controlled repository")
	}

	repoPath, err := vcs.GetRepoRoot(cwd)
	if err != nil {
		return err
	}
	sourceBranch, err := vcs.GetCurrentBranch(repoPath)
	if err != nil {
		return err
	}
	id := generateID(sm)

	// Default worktree ID to nt-<id>, skipping if branch/dir already exists
	if worktreeID == "" {
		n, _ := strconv.Atoi(id)
		for {
			candidate := fmt.Sprintf("nt-%d", n)
			dirExists := false
			if _, err := os.Stat(filepath.Join(repoPath, ".nanotown", candidate)); err == nil {
				dirExists = true
			}
			if !dirExists && !vcs.BranchExists(repoPath, candidate) {
				worktreeID = candidate
				break
			}
			n++
		}
	}

	// Check if worktree already exists; reuse if so, otherwise create
	wtPath := filepath.Join(repoPath, ".nanotown", worktreeID)
	if _, err := os.Stat(wtPath); err == nil {
		fmt.Printf("Reusing existing worktree: %s\n", worktreeID)
	} else {
		wtPath, err = vcs.CreateWorkingCopy(repoPath, worktreeID)
		if err != nil {
			return err
		}
	}

	// Record worktree metadata that persists across session deletions
	os.WriteFile(filepath.Join(wtPath, ".nt-source-branch"), []byte(sourceBranch), 0644)
	if desc != "" {
		os.WriteFile(filepath.Join(wtPath, ".nt-description"), []byte(desc), 0644)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	session := &Session{
		ID:              id,
		RepoPath:        repoPath,
		WorkingCopyPath: wtPath,
		VcsBackendName:  vcs.Name(),
		Alive:           true,
		PID:             -1,
		StartedAt:       now,
		LastOutputAt:    now,
		Worktree:        worktreeID,
	}
	if err := sm.Write(session); err != nil {
		return err
	}

	// Set env vars so the session ID is discoverable from inside
	os.Setenv("NT_SESSION", id)
	os.Setenv("NT_BRANCH", worktreeID)
	defer os.Unsetenv("NT_SESSION")
	defer os.Unsetenv("NT_BRANCH")

	// Write banner to a temp file so the shell can display it on startup
	bannerFile := filepath.Join(wtPath, ".nt-banner")
	os.WriteFile(bannerFile, []byte(formatBanner(id, worktreeID, desc)), 0644)
	defer os.Remove(bannerFile)

	bridge := &PtyBridge{
		session:        session,
		sessionManager: sm,
	}
	title := formatTitle(worktreeID, desc)
	if err := bridge.Launch(wtPath, bannerFile, title); err != nil {
		return fmt.Errorf("failed to launch PTY process: %w", err)
	}

	session.PID = bridge.Pid()
	if err := sm.Write(session); err != nil {
		return err
	}

	bridge.WaitFor()

	session.Alive = false
	sm.Write(session) // best-effort
	fmt.Printf("Session %s exited.\n", id)
	return nil
}

func cmdLiveStatus(sm *SessionManager) error {
	// Hide cursor
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	var sessions []*Session
	var worktrees []worktreeInfo
	prevLines := 0
	tick := 0
	for {
		// Re-read session files and run expensive operations every 1s (every 10th tick)
		if tick%10 == 0 {
			sessions = sm.ListAll()
			worktrees = listWorktrees(sessions)
			refreshSessionInfo(sessions, sm, worktrees)
		}

		output, lines := renderStatus(sessions, worktrees)

		// Move cursor up to overwrite previous frame
		if prevLines > 0 {
			fmt.Printf("\033[%dA\r", prevLines)
		}

		// Print each line, clearing to end of line to remove stale chars
		outputLines := strings.Split(output, "\n")
		for i, line := range outputLines {
			fmt.Printf("%s\033[K", line)
			if i < len(outputLines)-1 {
				fmt.Print("\n")
			}
		}

		// Clear any extra lines from previous frame
		for i := lines; i < prevLines; i++ {
			fmt.Print("\n\033[K")
			lines++
		}

		// Move to next line so cursor is below the output
		fmt.Print("\n")

		prevLines = lines
		tick++
		time.Sleep(100 * time.Millisecond)
	}
}

func stopSession(session *Session, sm *SessionManager) {
	if !session.Alive || !isProcessAlive(session.PID) {
		return
	}
	fmt.Printf("Stopping session %s (pid %d)...\n", session.ID, session.PID)
	killProcess(session.PID)
	time.Sleep(3 * time.Second)
	if isProcessAlive(session.PID) {
		fmt.Println("Process still alive, force killing...")
		forceKillProcess(session.PID)
	}
	session.Alive = false
	sm.Write(session) // best-effort
}

func cmdStop(worktreeID string, sm *SessionManager, cwd string) error {
	stopped := 0
	for _, s := range sm.ListAll() {
		wt := resolveWorktreeID(s)
		if wt == worktreeID && s.Alive && isProcessAlive(s.PID) {
			stopSession(s, sm)
			stopped++
		}
	}
	if stopped == 0 {
		// Check if the worktree even exists
		vcs := detectVcs(cwd)
		if vcs != nil {
			repoPath, _ := vcs.GetRepoRoot(cwd)
			if repoPath != "" {
				wtPath := filepath.Join(repoPath, ".nanotown", worktreeID)
				if _, err := os.Stat(wtPath); err != nil {
					return fmt.Errorf("worktree not found: %s", worktreeID)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "No running sessions on worktree %s.\n", worktreeID)
		return nil
	}
	fmt.Printf("Stopped %d session(s) on worktree %s.\n", stopped, worktreeID)
	return nil
}

func cmdRm(worktreeID string, sm *SessionManager, cwd string) error {
	vcs := detectVcs(cwd)
	if vcs == nil {
		return fmt.Errorf("not inside a version-controlled repository")
	}
	repoPath, err := vcs.GetRepoRoot(cwd)
	if err != nil {
		return err
	}

	wtPath := filepath.Join(repoPath, ".nanotown", worktreeID)
	if _, err := os.Stat(wtPath); err != nil {
		return fmt.Errorf("worktree not found: %s", worktreeID)
	}

	// Stop all sessions on this worktree and delete them
	for _, s := range sm.ListAll() {
		wt := resolveWorktreeID(s)
		if wt == worktreeID {
			stopSession(s, sm)
			sm.Delete(s.ID)
		}
	}

	vcs.RemoveWorkingCopy(repoPath, worktreeID)
	fmt.Printf("Worktree %s deleted.\n", worktreeID)
	return nil
}

func cmdMerge(target string, sm *SessionManager, cwd string) error {
	vcs := detectVcs(cwd)
	if vcs == nil {
		return fmt.Errorf("not inside a version-controlled repository")
	}
	repoPath, err := vcs.GetRepoRoot(cwd)
	if err != nil {
		return err
	}

	// Check for clean working directory
	status, err := runCommand(repoPath, "git", "status", "--porcelain")
	if err == nil && status != "" {
		return fmt.Errorf("working directory is not clean. Commit or stash your changes first")
	}

	currentBranch, err := vcs.GetCurrentBranch(repoPath)
	if err != nil {
		return err
	}
	wtPath := filepath.Join(repoPath, ".nanotown", target)

	// Check if any running sessions use this worktree
	for _, s := range sm.ListAll() {
		wt := resolveWorktreeID(s)
		if wt == target && s.Alive && isProcessAlive(s.PID) {
			return fmt.Errorf("session %s is still running on worktree %s. Stop it first with: nt stop %s", s.ID, target, target)
		}
	}

	// Read source branch from worktree metadata
	sourceBranch := ""
	data, _ := os.ReadFile(filepath.Join(wtPath, ".nt-source-branch"))
	if len(data) > 0 {
		sourceBranch = strings.TrimSpace(string(data))
	}

	// Warn if source branch differs from current branch
	if sourceBranch != "" && sourceBranch != currentBranch {
		fmt.Printf("Warning: worktree %s was created from branch %q, but current branch is %q.\n", target, sourceBranch, currentBranch)
		fmt.Print("Merge into current branch anyway? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return nil
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if !confirmMerge(wtPath, currentBranch, target) {
		return nil
	}
	if vcs.Merge(repoPath, currentBranch, target) {
		vcs.RemoveWorkingCopy(repoPath, target)
		// Clean up any sessions that used this worktree
		for _, s := range sm.ListAll() {
			wt := resolveWorktreeID(s)
			if wt == target {
				sm.Delete(s.ID)
			}
		}
		fmt.Printf("Merged worktree %s into %s and cleaned up.\n", target, currentBranch)
	} else {
		fmt.Fprintf(os.Stderr, "Merge conflict. Resolve conflicts in the repo, then run: nt merge %s again\n", target)
	}
	return nil
}

// confirmMerge checks for uncommitted changes and no-op merges.
// Returns true if the merge should proceed.
func confirmMerge(wtPath string, sourceBranch string, branch string) bool {
	// Check if worktree has uncommitted changes
	if _, err := os.Stat(wtPath); err == nil {
		status, err := runCommand(wtPath, "git", "status", "--porcelain")
		if err == nil && status != "" {
			fmt.Printf("Warning: worktree %s has uncommitted changes:\n%s\n", branch, status)
			fmt.Print("Merge anyway? Uncommitted changes will be lost. [y/N] ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return false
			}
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return false
			}
		}
	}

	// Check if there are any commits to merge
	revs, err := runCommand(wtPath, "git", "rev-list", "--count", sourceBranch+".."+branch)
	if err == nil && strings.TrimSpace(revs) == "0" {
		fmt.Printf("Nothing to merge — %s has no new commits vs %s.\n", branch, sourceBranch)
		return false
	}

	return true
}

func cmdStopAll(sm *SessionManager, cwd string) error {
	vcs := detectVcs(cwd)
	if vcs == nil {
		return fmt.Errorf("not inside a version-controlled repository")
	}
	repoPath, err := vcs.GetRepoRoot(cwd)
	if err != nil {
		return err
	}
	sessions := sm.ListForRepo(repoPath)

	stopped := 0
	for _, s := range sessions {
		if s.Alive && isProcessAlive(s.PID) {
			stopSession(s, sm)
			stopped++
		}
	}
	fmt.Printf("Stopped %d session(s).\n", stopped)
	return nil
}

func cmdAutoClean(sm *SessionManager) error {
	sessions := sm.ListAll()

	cleaned := 0
	skipped := 0
	for _, s := range sessions {
		if s.Alive && isProcessAlive(s.PID) {
			continue // skip running sessions
		}

		// Mark as stopped if still marked alive but process is no longer running
		if s.Alive {
			s.Alive = false
			sm.Write(s) // best-effort
		}

		// Check if worktree has uncommitted changes
		if s.WorkingCopyPath != "" {
			if _, err := os.Stat(s.WorkingCopyPath); err == nil {
				status, err := runCommand(s.WorkingCopyPath, "git", "status", "--porcelain")
				if err == nil && status != "" {
					fmt.Printf("Skipping session %s — worktree has uncommitted changes\n", s.ID)
					skipped++
					continue
				}
			}
		}

		wt := resolveWorktreeID(s)
		// Only remove worktree if no other session still references it
		otherUsing := false
		for _, other := range sessions {
			if other.ID == s.ID {
				continue
			}
			owt := resolveWorktreeID(other)
			if owt == wt && other.Alive && isProcessAlive(other.PID) {
				otherUsing = true
				break
			}
		}

		vcs := detectVcs(s.RepoPath)
		if vcs != nil && !otherUsing {
			vcs.RemoveWorkingCopy(s.RepoPath, wt)
		}
		sm.Delete(s.ID)
		fmt.Printf("Cleaned session %s\n", s.ID)
		cleaned++
	}

	// Scan for orphaned worktree directories not referenced by any remaining session
	remainingSessions := sm.ListAll()
	// Rebuild referenced set from remaining sessions
	referenced := map[string]bool{}
	repoPathForOrphan := ""
	for _, s := range remainingSessions {
		wt := resolveWorktreeID(s)
		referenced[wt] = true
		repoPathForOrphan = s.RepoPath
	}

	// Also try to detect repo path from cwd if no sessions remain
	if repoPathForOrphan == "" {
		cwd, _ := os.Getwd()
		vcs := detectVcs(cwd)
		if vcs != nil {
			repoPathForOrphan, _ = vcs.GetRepoRoot(cwd)
		}
	}

	if repoPathForOrphan != "" {
		ntDir := filepath.Join(repoPathForOrphan, ".nanotown")
		entries, err := os.ReadDir(ntDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				if referenced[name] {
					continue
				}
				wtPath := filepath.Join(ntDir, name)
				// Check for uncommitted changes
				status, err := runCommand(wtPath, "git", "status", "--porcelain")
				if err == nil && status != "" {
					fmt.Printf("Skipping orphaned worktree %s — has uncommitted changes\n", name)
					skipped++
					continue
				}
				vcs := detectVcs(repoPathForOrphan)
				if vcs != nil {
					vcs.RemoveWorkingCopy(repoPathForOrphan, name)
					fmt.Printf("Cleaned orphaned worktree %s\n", name)
					cleaned++
				}
			}
		}
	}

	if cleaned == 0 && skipped == 0 {
		fmt.Println("Nothing to clean.")
	} else {
		fmt.Printf("Cleaned %d session(s)/worktree(s), skipped %d with uncommitted changes.\n", cleaned, skipped)
	}
	return nil
}

func cmdDeleteAll(sm *SessionManager, cwd string) error {
	vcs := detectVcs(cwd)
	if vcs == nil {
		return fmt.Errorf("not inside a version-controlled repository")
	}
	repoPath, err := vcs.GetRepoRoot(cwd)
	if err != nil {
		return err
	}
	sessions := sm.ListForRepo(repoPath)

	if len(sessions) == 0 {
		fmt.Println("No sessions to clean.")
		return nil
	}

	running := 0
	exited := 0
	for _, s := range sessions {
		if s.Alive && isProcessAlive(s.PID) {
			running++
		} else {
			exited++
		}
	}

	fmt.Printf("This will delete %d session(s) and their worktrees.\n", len(sessions))
	if running > 0 {
		fmt.Printf("  %d running session(s) will be stopped.\n", running)
	}
	fmt.Print("Continue? [y/N] ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	cleaned := 0
	removedWorktrees := map[string]bool{}
	for _, s := range sessions {
		stopSession(s, sm)
		wt := resolveWorktreeID(s)
		if !removedWorktrees[wt] {
			vcs.RemoveWorkingCopy(repoPath, wt)
			removedWorktrees[wt] = true
		}
		sm.Delete(s.ID)
		cleaned++
	}

	// Also remove any orphaned worktree directories not referenced by sessions
	ntDir := filepath.Join(repoPath, ".nanotown")
	entries, err := os.ReadDir(ntDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if removedWorktrees[name] {
				continue
			}
			vcs.RemoveWorkingCopy(repoPath, name)
			cleaned++
		}
	}

	fmt.Printf("Cleaned %d session(s)/worktree(s).\n", cleaned)
	return nil
}

func formatBanner(id, worktreeID, desc string) string {
	var b strings.Builder
	b.WriteString("\n  nanotown session started\n")
	fmt.Fprintf(&b, "  session %-6s worktree %s\n", id, worktreeID)
	if desc != "" {
		fmt.Fprintf(&b, "  %s\n", desc)
	}
	b.WriteString("\n  Type exit to end the session.\n\n")
	return b.String()
}

func formatTitle(worktreeID, desc string) string {
	title := fmt.Sprintf("[%s]", worktreeID)
	if desc != "" {
		title += " " + desc
	}
	return title
}

func readWorktreeDesc(wtPath string) string {
	data, err := os.ReadFile(filepath.Join(wtPath, ".nt-description"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
