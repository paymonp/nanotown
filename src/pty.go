package main

import (
	"io"
	"os"
	"time"
)

type PtyBridge struct {
	session        *Session
	sessionManager *SessionManager
	pid            int
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

func (b *PtyBridge) startIO() {
	// PTY output -> real stdout
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := b.ptyReader.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
				if hasPrintableContent(buf, n) {
					b.session.LastOutputAt = time.Now().UTC().Format(time.RFC3339Nano)
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
			// 0x40-0x7E terminate standard escape sequences; 0x07 (BEL) terminates OSC sequences
			if (b >= 0x40 && b <= 0x7E) || b == 0x07 {
				inEscape = false
			}
			continue
		}
		if b >= 0x20 && b != 0x7F { // printable ASCII or UTF-8 continuation bytes
			return true
		}
	}
	return false
}
