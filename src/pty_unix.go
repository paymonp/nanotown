//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func (b *PtyBridge) Launch(workDir string, bannerFile string, title string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Build the exec line with prompt customization for supported shells.
	// Green [nt] prefix, similar to how PS shows before the dir in PowerShell.
	shellBase := filepath.Base(shell)
	execLine := fmt.Sprintf(`exec "%s"`, shell)

	switch shellBase {
	case "bash":
		metaDir := filepath.Join(workDir, ".nanotown")
		os.MkdirAll(metaDir, 0755)
		rcFile := filepath.Join(metaDir, "bashrc")
		os.WriteFile(rcFile, []byte(
			"[ -f ~/.bashrc ] && . ~/.bashrc\n"+
				`PS1="\[\033[32m\][nt]\[\033[0m\] $PS1"`+"\n",
		), 0644)
		execLine = fmt.Sprintf(`exec "%s" --rcfile "%s"`, shell, rcFile)
	case "zsh":
		// zsh reads dotfiles from ZDOTDIR instead of $HOME when set
		zdotdir := filepath.Join(workDir, ".nanotown", "zsh")
		os.MkdirAll(zdotdir, 0755)
		os.WriteFile(filepath.Join(zdotdir, ".zshenv"), []byte(
			"[ -f \"$HOME/.zshenv\" ] && . \"$HOME/.zshenv\"\n",
		), 0644)
		os.WriteFile(filepath.Join(zdotdir, ".zshrc"), []byte(
			"[ -f \"$HOME/.zshrc\" ] && . \"$HOME/.zshrc\"\n"+
				"PS1=\"%F{green}[nt]%f $PS1\"\n",
		), 0644)
		execLine = fmt.Sprintf(`ZDOTDIR="%s" exec "%s"`, zdotdir, shell)
	}

	// Build launch script: optional banner + optional title + exec into user's shell
	var parts []string
	if bannerFile != "" {
		parts = append(parts, `cat "$1"; rm -f "$1"`)
	}
	if title != "" {
		parts = append(parts, fmt.Sprintf(`printf '\033]0;%s\007'`, title))
	}
	parts = append(parts, execLine)
	script := strings.Join(parts, "; ")

	var cmd *exec.Cmd
	if bannerFile != "" {
		cmd = exec.Command("/bin/sh", "-c", script, "--", bannerFile)
	} else {
		cmd = exec.Command("/bin/sh", "-c", script)
	}
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	var ws *pty.Winsize
	w, h, sizeErr := term.GetSize(int(os.Stdout.Fd()))
	if sizeErr == nil {
		ws = &pty.Winsize{Cols: uint16(w), Rows: uint16(h)}
	}
	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return err
	}

	b.pid = cmd.Process.Pid
	b.ptyReader = ptmx
	b.ptyWriter = ptmx

	oldState, termErr := term.MakeRaw(int(os.Stdin.Fd()))

	b.waitFunc = func() {
		cmd.Wait()
	}
	b.cleanupFunc = func() {
		ptmx.Close()
		if termErr == nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
	}

	b.startIO()
	return nil
}
