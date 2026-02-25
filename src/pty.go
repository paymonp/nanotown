package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type PtyBridge struct {
	session        *Session
	sessionManager *SessionManager
	pid            int
	termHeight     int
	statusBar      string
	ptyReader      io.Reader
	ptyWriter      io.Writer
	waitFunc       func()
	cleanupFunc    func()
}

func (b *PtyBridge) WaitFor() {
	if b.waitFunc != nil {
		b.waitFunc()
	}
	if b.cleanupFunc != nil {
		b.cleanupFunc()
	}
}

func (b *PtyBridge) Pid() int {
	return b.pid
}

func (b *PtyBridge) drawStatusBar() {
	if b.termHeight <= 0 || b.statusBar == "" {
		return
	}
	// Save cursor, jump to last row, draw bar in reverse video, restore cursor
	bar := fmt.Sprintf("\0337\033[%d;1H\033[7m %s \033[0m\033[K\0338", b.termHeight, b.statusBar)
	os.Stdout.WriteString(bar)
}

func (b *PtyBridge) startIO() {
	// Draw initial status bar
	b.drawStatusBar()

	// PTY output -> real stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := b.ptyReader.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
				b.drawStatusBar()
				if hasPrintableContent(buf, n) {
					b.session.LastOutputAt = time.Now().UTC().Format(time.RFC3339Nano)
					if line := extractLastLine(buf[:n]); line != "" {
						b.session.LastOutputLine = line
					}
					b.sessionManager.Write(b.session) // best-effort
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// Real stdin -> PTY input
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				b.ptyWriter.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()
}

// stripAnsi removes ANSI escape sequences and carriage returns from raw bytes.
func stripAnsi(data []byte) string {
	var result []byte
	inEscape := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == 0x1B {
			inEscape = true
			continue
		}
		if inEscape {
			if (b >= 0x40 && b <= 0x7E) || b == 0x07 {
				inEscape = false
			}
			continue
		}
		if b == '\r' {
			continue
		}
		result = append(result, b)
	}
	return string(result)
}

// extractLastLine strips ANSI from a raw output chunk and returns the last
// non-empty line, truncated to 120 chars.
func extractLastLine(data []byte) string {
	cleaned := stripAnsi(data)
	lines := strings.Split(cleaned, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if len(line) > 120 {
				line = line[:120]
			}
			return line
		}
	}
	return ""
}

// hasPrintableContent returns true if the buffer contains at least one
// printable character, filtering out pure ANSI escape sequences.
func hasPrintableContent(buf []byte, length int) bool {
	inEscape := false
	for i := 0; i < length; i++ {
		b := buf[i]
		if b == 0x1B {
			inEscape = true
			continue
		}
		if inEscape {
			if (b >= 0x40 && b <= 0x7E) || b == 0x07 {
				inEscape = false
			}
			continue
		}
		if b >= 0x20 && b != 0x7F {
			return true
		}
	}
	return false
}
