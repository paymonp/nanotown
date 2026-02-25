package main

type VcsBackend interface {
	Detect(path string) bool
	GetRepoRoot(cwd string) (string, error)
	GetCurrentBranch(repoPath string) (string, error)
	BranchExists(repoPath string, branch string) bool
	CreateWorkingCopy(repoPath string, worktreeID string) (string, error)
	Merge(repoPath string, sourceBranch string, branch string) bool
	RemoveWorkingCopy(repoPath string, worktreeID string)
}

var vcsBackends = []VcsBackend{
	&GitBackend{},
}

func detectVcs(cwd string) VcsBackend {
	for _, b := range vcsBackends {
		if b.Detect(cwd) {
			return b
		}
	}
	return nil
}
