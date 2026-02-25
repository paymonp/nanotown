//go:build windows

package main

import (
	"context"
	"os"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func (b *PtyBridge) Launch(command string, workDir string) error {
	// Enable virtual terminal processing on stdout
	hStdout, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	var savedOutputMode uint32
	hasOutput := windows.GetConsoleMode(hStdout, &savedOutputMode) == nil
	if hasOutput {
		windows.SetConsoleMode(hStdout, savedOutputMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}

	// Set stdin to raw mode
	oldState, termErr := term.MakeRaw(int(os.Stdin.Fd()))

	// Launch via ConPTY — reserve bottom row for status bar
	opts := []conpty.ConPtyOption{conpty.ConPtyWorkDir(workDir)}
	w, h, sizeErr := term.GetSize(int(os.Stdout.Fd()))
	if sizeErr == nil {
		b.termHeight = h
		opts = append(opts, conpty.ConPtyDimensions(w, h-1))
	}
	cpty, err := conpty.Start("cmd.exe /c "+command, opts...)
	if err != nil {
		if termErr == nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
		if hasOutput {
			windows.SetConsoleMode(hStdout, savedOutputMode)
		}
		return err
	}

	b.pid = cpty.Pid()
	b.ptyReader = cpty
	b.ptyWriter = cpty

	b.waitFunc = func() {
		cpty.Wait(context.Background())
	}
	b.cleanupFunc = func() {
		cpty.Close()
		if termErr == nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
		}
		if hasOutput {
			windows.SetConsoleMode(hStdout, savedOutputMode)
		}
	}

	b.startIO()
	return nil
}
