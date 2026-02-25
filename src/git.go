package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const worktreeDir = ".nanotown"

type GitBackend struct{}

func (g *GitBackend) Name() string {
	return "git"
}

func (g *GitBackend) Detect(path string) bool {
	current := path
	for {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && (info.IsDir() || !info.IsDir()) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return false
}

func (g *GitBackend) GetRepoRoot(cwd string) (string, error) {
	result, err := runCommand(cwd, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("failed to get repo root: %w", err)
	}
	return result, nil
}

func (g *GitBackend) GetCurrentBranch(repoPath string) (string, error) {
	result, err := runCommand(repoPath, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("this repository has no commits yet — make an initial commit before using nanotown")
	}
	return result, nil
}

func (g *GitBackend) CreateWorkingCopy(repoPath string, worktreeID string) (string, error) {
	wtPath := filepath.Join(repoPath, worktreeDir, worktreeID)
	// Try creating with a new branch first; if the branch already exists, reuse it
	_, err := runCommand(repoPath, "git", "worktree", "add", wtPath, "-b", worktreeID)
	if err != nil {
		_, err = runCommand(repoPath, "git", "worktree", "add", wtPath, worktreeID)
		if err != nil {
			return "", fmt.Errorf("failed to create worktree %q: %w", worktreeID, err)
		}
	}
	return wtPath, nil
}

// BranchExists checks if a git branch exists.
func (g *GitBackend) BranchExists(repoPath string, branch string) bool {
	_, err := runCommand(repoPath, "git", "rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

func (g *GitBackend) Merge(repoPath string, sourceBranch string, branch string) bool {
	_, err := runCommand(repoPath, "git", "checkout", sourceBranch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Merge conflict or failure: %s\n", err)
		return false
	}
	_, err = runCommand(repoPath, "git", "merge", branch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Merge conflict or failure: %s\n", err)
		return false
	}
	return true
}

func (g *GitBackend) RemoveWorkingCopy(repoPath string, worktreeID string) {
	wtPath := filepath.Join(repoPath, worktreeDir, worktreeID)
	runCommand(repoPath, "git", "worktree", "remove", wtPath)
	runCommand(repoPath, "git", "branch", "-d", worktreeID)
}
