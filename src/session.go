package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Session struct {
	ID              string `json:"id"`
	Model           string `json:"model"`
	RepoPath        string `json:"repoPath"`
	WorkingCopyPath string `json:"workingCopyPath"`
	SourceBranch    string `json:"sourceBranch,omitempty"` // cache for display; authoritative source is .nanotown/source-branch
	Alive           bool   `json:"alive"`
	PID             int    `json:"pid"`
	StartedAt       string `json:"startedAt"`
	LastOutputAt    string `json:"lastOutputAt"`
	Worktree        string `json:"worktree,omitempty"` // fallback; prefer resolveWorktreeID() which uses WorkingCopyPath
}

type SessionManager struct {
	dir string
}

func NewSessionManager() (*SessionManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(home, ".nanotown", "sessions")
	os.MkdirAll(dir, 0755)
	return &SessionManager{dir: dir}, nil
}

func (sm *SessionManager) Write(s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	path := filepath.Join(sm.dir, s.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	return nil
}

func (sm *SessionManager) ListAll() []*Session {
	var sessions []*Session
	entries, err := os.ReadDir(sm.dir)
	if err != nil {
		return sessions
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sm.dir, entry.Name()))
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, &s)
	}
	return sessions
}

func (sm *SessionManager) ListForRepo(repoPath string) []*Session {
	all := sm.ListAll()
	var result []*Session
	for _, s := range all {
		if s.RepoPath == repoPath {
			result = append(result, s)
		}
	}
	return result
}

func (sm *SessionManager) Delete(id string) {
	path := filepath.Join(sm.dir, id+".json")
	os.Remove(path)
}

