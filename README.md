<p align="center">
  <img src="logo.png" alt="nanotown" width="450" />
  <h1>nanotown</h1>
</p>

A simple way to run and manage multiple AI agents on the same machine. No servers or databases.

Each session gets its own isolated VCS worktree and its own terminal. You check on them with `nt status`, and merge their work back when they're done.

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
nt "fix the auth bug"
```

This creates an isolated worktree and drops you into an interactive shell inside it. Run whatever agent you want — Claude Code, Aider, OpenCode, or anything else. nanotown is just the worktree and session manager.

Use `-w <worktree-id>` to specify a custom worktree ID. Worktrees are reused if they already exist, and multiple sessions can share one. Without `-w`, each session gets its own auto-generated worktree ID (`nt-1`, `nt-2`, etc).

### Check on your sessions

```
nt status
```

```
Sessions
REPO             BRANCH     SESSION MODEL      STATUS       WORKTREE   STARTED    LAST ACTIVE  DESCRIPTION
user/myproject   main       1       claude     ⠋ active     auth-bug   2m ago     1m ago       fix the auth bug
user/myproject   main       2       kimi       ⠋ idle 45s   auth-fix   15m ago    5m ago       add tests
user/myproject   main       3       —          exited       refactor   1h ago     45m ago      refactor db layer
user/myproject   main       4       —          exited       refactor   1h ago     45m ago      refactor auth layer

Worktrees
REPO             BRANCH     WORKTREE   SESSIONS   DESCRIPTION
user/myproject   main       auth-bug   1          fix the auth bug
user/myproject   main       auth-fix   2          add tests
user/myproject   main       refactor   3, 4       refactor db layer
```

Live-updating display. nanotown auto-detects running agents (Claude Code, Aider, OpenCode, etc.) for the MODEL column. Sessions and worktrees from all repos are shown.

### Merge work back

```
nt merge auth-bug
```

Merges a worktree into your current VCS branch. If the worktree was created from a different branch, you'll get a warning before proceeding.

## Commands

```
Lifecycle:
  nt <desc>                     Launch a session
  nt -w <worktree-id> [desc]    Launch a session with a custom worktree ID
  nt status                     Show all sessions (live-updating)
  nt merge <worktree-id>        Merge into your current VCS branch and clean up

Cleanup:
  nt stop <worktree-id>         Stop all running sessions on a worktree
  nt stopall                    Stop all running sessions
  nt clean                      Remove stopped sessions and orphaned worktrees
  nt delete <worktree-id>       Delete a worktree and its sessions
  nt deleteall                  Delete all sessions and worktrees
```

## How it works

Each session gets its own git worktree and branch under `.nanotown/` in your repo. The agent runs inside it via a PTY with full terminal passthrough. When done, `nt merge` brings the work back into your current branch.

No daemon, no background process, no database.

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

[MIT](LICENSE.md)
