package auth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// TerminalSession represents an active PTY session for a CLI auth command.
type TerminalSession struct {
	ID         string
	EngineType string // "opencode"
	Status     string // "running", "completed", "failed"
	ptmx       *os.File
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	mu         sync.Mutex
}

// activeSession tracks the single allowed terminal session.
var (
	activeSession   *TerminalSession
	activeSessionMu sync.Mutex
)

// StartTerminalSession spawns the auth command in a PTY.
// Only one session is allowed at a time; any existing session is force-closed.
func StartTerminalSession(ctx context.Context, engineType string) (*TerminalSession, error) {
	// Grab and clear any existing session.
	activeSessionMu.Lock()
	old := activeSession
	activeSession = nil
	activeSessionMu.Unlock()

	if old != nil {
		old.Close()
	}

	var cmdName string
	var cmdArgs []string

	switch engineType {
	case "opencode":
		cmdName = "opencode"
		cmdArgs = []string{"auth", "login"}
	default:
		return nil, fmt.Errorf("unsupported engine type: %s", engineType)
	}

	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	// Ensure HOME is set correctly — Bun/OpenCode reads the passwd entry which
	// may be /nonexistent for system users, even if the HOME env var is set.
	home := os.Getenv("HOME")
	if home == "" || home == "/nonexistent" {
		home = "/home/openpact-system"
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "HOME="+home)

	// Start command in a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	session := &TerminalSession{
		ID:         fmt.Sprintf("auth-%s-%d", engineType, os.Getpid()),
		EngineType: engineType,
		Status:     "running",
		ptmx:       ptmx,
		cmd:        cmd,
		cancel:     cancel,
	}

	activeSessionMu.Lock()
	activeSession = session
	activeSessionMu.Unlock()

	return session, nil
}

// Read reads from the PTY master (CLI output).
func (s *TerminalSession) Read(buf []byte) (int, error) {
	return s.ptmx.Read(buf)
}

// Write writes to the PTY master (user input).
func (s *TerminalSession) Write(data []byte) (int, error) {
	return s.ptmx.Write(data)
}

// Resize sends a window size change to the PTY.
func (s *TerminalSession) Resize(rows, cols uint16) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Close kills the subprocess and cleans up the PTY. Safe to call multiple times.
func (s *TerminalSession) Close() error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.ptmx != nil {
		s.ptmx.Close()
		s.ptmx = nil
	}
	s.Status = "completed"
	s.mu.Unlock()

	activeSessionMu.Lock()
	if activeSession == s {
		activeSession = nil
	}
	activeSessionMu.Unlock()

	return nil
}

// Wait blocks until the subprocess exits and updates the session status.
func (s *TerminalSession) Wait() error {
	err := s.cmd.Wait()

	s.mu.Lock()
	if err != nil {
		s.Status = "failed"
	} else {
		s.Status = "completed"
	}
	s.mu.Unlock()

	activeSessionMu.Lock()
	if activeSession == s {
		activeSession = nil
	}
	activeSessionMu.Unlock()

	return err
}
