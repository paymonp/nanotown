//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func (b *PtyBridge) Launch(workDir string, bannerFile string, title string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	var cmd *exec.Cmd
	if bannerFile != "" {
		// Print banner, set title, delete file, then exec into the user's shell
		script := fmt.Sprintf(`cat "$1"; rm -f "$1"; printf '\033]0;%s\007'; exec "$2"`, title)
		cmd = exec.Command("/bin/sh", "-c", script, "--", bannerFile, shell)
	} else if title != "" {
		script := fmt.Sprintf(`printf '\033]0;%s\007'; exec "$1"`, title)
		cmd = exec.Command("/bin/sh", "-c", script, "--", shell)
	} else {
		cmd = exec.Command(shell)
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
