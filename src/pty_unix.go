//go:build !windows

package main

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func (b *PtyBridge) Launch(command string, workDir string) error {
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	// Start with terminal size — reserve bottom row for status bar
	var ws *pty.Winsize
	w, h, sizeErr := term.GetSize(int(os.Stdout.Fd()))
	if sizeErr == nil {
		b.termHeight = h
		ws = &pty.Winsize{Cols: uint16(w), Rows: uint16(h - 1)}
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
