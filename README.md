<p align="center">
  <img src="logo.png" alt="nanotown" width="450" />
  <h1>nanotown</h1>
</p>

A simple way to run and manage multiple AI agents on the same machine. No servers or databases.

Each agent gets its own isolated VCS worktree and its own terminal. You check on them with `nt status`, and merge their work back when they're done.

## Install

```
curl -fsSL https://raw.githubusercontent.com/paymonp/nanotown/main/install.sh | sh
```

On Windows (PowerShell):

```
irm https://raw.githubusercontent.com/paymonp/nanotown/main/install.ps1 | iex
```

No runtime dependencies. Downloads a single native binary for your platform.

## Usage

### Start a session

```
nt claude "fix the auth bug"
```

This creates an isolated worktree, launches the agent inside it via a PTY, and gives you a fully interactive terminal session. You talk to the agent directly — nanotown is invisible.

Use `-w <id>` to specify a custom worktree ID. Worktrees are reused if they already exist, and multiple sessions can share one. Without `-w`, each session gets its own auto-generated worktree ID.

### Check on your sessions

```
nt status
```

```
SESSION MODEL      STATUS       BRANCH     WORKTREE   REPO             STARTED    LAST ACTIVE  DESCRIPTION
1       claude     active       main       auth-bug   user/myproject   2m ago     1m ago       fix the auth bug
2       kimi       idle 45s     main       auth-fix   user/myproject   15m ago    5m ago       add tests
3       codex      exited       main       refactor   user/myproject   1h ago     45m ago      refactor db layer
4       codex      exited       main       refactor   user/myproject   1h ago     45m ago      refactor auth layer

BRANCH     WORKTREE   SESSIONS
main       auth-bug   1
main       auth-fix   2
main       refactor   3, 4
```

### Merge work back

```
nt merge auth-bug
```

Merges a worktree into your current VCS branch. If the worktree was created from a different branch, you'll get a warning before proceeding.

## How it works

Each session gets its own git worktree and branch under `.nanotown/` in your repo. The agent runs inside it via a PTY with full terminal passthrough. When done, `nt merge` brings the work back into your current branch.

No daemon, no background process, no database.

## Supported agents

| Agent | Command |
|-------|---------|
| Claude Code | `claude` |
| OpenCode | `opencode` |
| Aider | `aider` |
| Kimi | `kimi` |
| Codex | `codex` |

The agent CLI must be installed separately on your system.

## .gitignore

Add this to projects using nanotown:

```
.nanotown/
```

## Build from source

Requires Go 1.21+.

```bash
go build -o nt ./src           # compile binary (Unix)
go build -o nt.exe ./src       # compile binary (Windows)
```

On Windows, `build_release.bat` automates local and release builds:

```
build_release.bat local    # builds to bin/nt.exe
build_release.bat 0.2.0    # builds to releases/0.2.0-1/
```

## License

[FSL-1.1-ALv2](LICENSE.md) — Functional Source License. Free to use, modify, and redistribute for any purpose except building a competing product. Converts to Apache 2.0 after two years.
