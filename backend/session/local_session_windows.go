//go:build windows
// +build windows

package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/UserExistsError/conpty"
)

type LocalSession struct {
	baseSession
	cpty     *conpty.ConPty
	stdin    io.WriteCloser
	stdout   io.Reader
	cmd      *exec.Cmd
	quit     chan struct{}
	quitOnce sync.Once
}

func NewLocalSession(id string) *LocalSession {
	s := &LocalSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "local",
			status:      StatusDisconnected,
		},
		quit: make(chan struct{}),
	}
	// Set a generous default size so the PTY is unlikely to scroll before
	// the frontend sends its first Resize() with the real dimensions.
	s.SetPendingSize(200, 60)
	return s
}

func (s *LocalSession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)

	shell := config.ShellPath
	if shell == "" {
		shell = defaultShell()
	}

	s.title = shellName(shell)

	// Try ConPTY first for a real pseudo-terminal experience.
	// This fixes MSYS2 bash (Git Bash) which requires a TTY for correct output.
	if conpty.IsConPtyAvailable() {
		commandLine := buildCommandLine(shell)
		cols, rows := s.GetPendingSize()
		if cols <= 0 || rows <= 0 {
			cols, rows = 80, 24
		}

		c, err := conpty.Start(commandLine, conpty.ConPtyDimensions(cols, rows))
		if err == nil {
			s.cpty = c

			go func() {
				_, _ = s.cpty.Wait(context.Background())
				s.Disconnect()
			}()

			s.setStatus(StatusConnected)
			go s.readLoop()
			return nil
		}
		// Fall through to pipe mode if ConPTY fails.
	}

	// Fallback pipe mode for older Windows without ConPTY.
	lowerShell := strings.ToLower(shell)
	if strings.Contains(lowerShell, "bash") {
		// WSL bash does not support --login -i passed this way.
		if strings.Contains(lowerShell, "system32") || strings.Contains(lowerShell, "wsl") {
			s.cmd = exec.Command(shell)
			s.cmd.Env = os.Environ()
		} else {
			s.cmd = exec.Command(shell, "--login", "-i")
			s.cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		}
	} else {
		s.cmd = exec.Command(shell)
		s.cmd.Env = os.Environ()
	}

	stdinPipe, err := s.cmd.StdinPipe()
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := s.cmd.StdoutPipe()
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("stdout pipe: %w", err)
	}

	s.cmd.Stderr = s.cmd.Stdout

	if err := s.cmd.Start(); err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("start command: %w", err)
	}

	s.stdin = stdinPipe
	s.stdout = stdoutPipe

	go func() {
		_ = s.cmd.Wait()
		s.Disconnect()
	}()

	s.setStatus(StatusConnected)
	go s.readLoop()

	return nil
}

func buildCommandLine(shell string) string {
	lower := strings.ToLower(shell)
	quoted := fmt.Sprintf(`"%s"`, shell)

	if strings.Contains(lower, "bash") {
		// WSL bash (inside System32) does not support --login -i passed this way.
		if strings.Contains(lower, "system32") || strings.Contains(lower, "wsl") {
			return quoted
		}
		return fmt.Sprintf(`"%s" --login -i`, shell)
	}
	if strings.Contains(lower, "cmd.exe") {
		return fmt.Sprintf(`"%s" /k`, shell)
	}
	return quoted
}

func (s *LocalSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.quit:
			return
		default:
		}

		var n int
		var err error
		if s.cpty != nil {
			n, err = s.cpty.Read(buf)
		} else {
			n, err = s.stdout.Read(buf)
		}

		if n > 0 {
			s.emitData(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			if err != io.EOF {
				s.emitData([]byte(fmt.Sprintf("\r\n[read error: %v]\r\n", err)))
			}
			s.Disconnect()
			return
		}
	}
}

func (s *LocalSession) Write(data []byte) error {
	if s.cpty != nil {
		_, err := s.cpty.Write(data)
		return err
	}
	if s.stdin != nil {
		_, err := s.stdin.Write(data)
		return err
	}
	return fmt.Errorf("not connected")
}

func (s *LocalSession) Disconnect() error {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	if s.cpty != nil {
		s.cpty.Close()
		s.cpty = nil
	}
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *LocalSession) Resize(cols, rows int) error {
	s.SetPendingSize(cols, rows)
	if s.cpty != nil {
		return s.cpty.Resize(cols, rows)
	}
	// Pipe mode: no resize support.
	return nil
}

func (s *LocalSession) IsConnected() bool {
	return s.Status() == StatusConnected
}

func defaultShell() string {
	if _, err := exec.LookPath("pwsh.exe"); err == nil {
		return "pwsh.exe"
	}
	if _, err := exec.LookPath("powershell.exe"); err == nil {
		return "powershell.exe"
	}
	// Prefer Git Bash over WSL bash to avoid WSL relay errors.
	gitBashPaths := []string{
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
	}
	for _, p := range gitBashPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if _, err := exec.LookPath("bash.exe"); err == nil {
		return "bash.exe"
	}
	return "cmd.exe"
}

func shellName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".exe")
	return base
}
