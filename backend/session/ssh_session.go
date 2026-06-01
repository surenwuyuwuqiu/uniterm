package session

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sshKeepAliveInterval = 30 * time.Second
	sshKeepAliveTimeout  = 10 * time.Second
	sshKeepAliveMaxFail  = 2
)

type SSHSession struct {
	baseSession
	client       *ssh.Client
	session      *ssh.Session
	stdin        io.WriteCloser
	stdout       io.Reader
	stderr       io.Reader
	quit         chan struct{}
	quitOnce     sync.Once
	lastReadTime atomic.Int64
}

func NewSSHSession(id string) *SSHSession {
	return &SSHSession{
		baseSession: baseSession{
			id:          id,
			sessionType: "ssh",
			status:      StatusDisconnected,
		},
		quit: make(chan struct{}),
	}
}

func (s *SSHSession) Connect(config ConnectionConfig) error {
	s.setStatus(StatusConnecting)
	s.title = fmt.Sprintf("%s@%s", config.User, config.Host)

	authMethods := makeSSHAuthMethods(config)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	clientConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		Timeout:         30 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := net.DialTimeout("tcp", addr, clientConfig.Timeout)
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("tcp dial: %w", err)
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(sshKeepAliveInterval)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		conn.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("ssh handshake: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("new session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	cols, rows := s.getInitialSize(80, 24)
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		session.Close()
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("request pty: %w", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("shell: %w", err)
	}

	go func() {
		_ = session.Wait()
		s.Disconnect()
	}()

	s.client = client
	s.session = session
	s.stdin = stdinPipe
	s.stdout = stdoutPipe
	s.stderr = stderrPipe
	s.setStatus(StatusConnected)

	// Apply pending terminal size if one was set before connection.
	if cols, rows := s.GetPendingSize(); cols > 0 && rows > 0 {
		_ = s.session.WindowChange(rows, cols)
	}

	go s.readLoop()
	go s.readStderr()
	go s.startKeepAlive()
	go s.runPostLoginScript(config.PostLoginScript)

	return nil
}

func (s *SSHSession) readStderr() {
	buf := make([]byte, 4096)
	for {
		n, err := s.stderr.Read(buf)
		if n > 0 {
			// Prefix stderr output so it can be distinguished in the UI
			data := append([]byte("\r\n\x1b[31m[stderr] \x1b[0m"), buf[:n]...)
			s.emitData(data)
		}
		if err != nil {
			return
		}
	}
}

func (s *SSHSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.stdout.Read(buf)
		if n > 0 {
			s.lastReadTime.Store(time.Now().UnixNano())
			s.emitData(append([]byte(nil), buf[:n]...))
		}
		if err != nil {
			if err != io.EOF {
				s.emitData([]byte(fmt.Sprintf("\r\n\x1b[31m[read error: %v]\x1b[0m\r\n", err)))
			} else {
				s.emitData([]byte("\r\n\x1b[31mConnection closed by remote host. Press Enter to reconnect.\x1b[0m\r\n"))
			}
			s.Disconnect()
			return
		}
	}
}

func (s *SSHSession) runPostLoginScript(script string) {
	if strings.TrimSpace(script) == "" {
		return
	}
	// Wait for shell to finish initialization (first idle period).
	if !s.waitIdle(5*time.Second, 300*time.Millisecond) {
		return
	}
	lines := strings.Split(script, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if s.Status() != StatusConnected {
			return
		}
		if s.stdin != nil {
			_, _ = s.stdin.Write([]byte(line + "\r"))
		}
		// Wait for command output to settle before sending the next line.
		s.waitIdle(3*time.Second, 300*time.Millisecond)
	}
}

// waitIdle blocks until stdout has been idle for the given duration,
// or until the overall timeout expires. It returns true on idle detection.
func (s *SSHSession) waitIdle(timeout, idle time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		last := time.Unix(0, s.lastReadTime.Load())
		if !last.IsZero() && time.Since(last) >= idle {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func (s *SSHSession) startKeepAlive() {
	ticker := time.NewTicker(sshKeepAliveInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ticker.C:
			if s.Status() != StatusConnected {
				return
			}

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
				done <- err
			}()

			select {
			case err := <-done:
				if err != nil {
					failures++
				} else {
					failures = 0
				}
			case <-time.After(sshKeepAliveTimeout):
				failures++
			}

			if failures >= sshKeepAliveMaxFail {
				s.emitData([]byte("\r\n\x1b[31mConnection lost. Press Enter to reconnect.\x1b[0m\r\n"))
				s.Disconnect()
				return
			}

		case <-s.quit:
			return
		}
	}
}

func (s *SSHSession) Write(data []byte) error {
	if s.stdin == nil {
		return fmt.Errorf("not connected")
	}
	_, err := s.stdin.Write(data)
	return err
}

func (s *SSHSession) Disconnect() error {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	if s.session != nil {
		s.session.Close()
	}
	if s.client != nil {
		s.client.Close()
	}
	s.setStatus(StatusDisconnected)
	return nil
}

func (s *SSHSession) Resize(cols, rows int) error {
	// Always save the desired size so it can be applied after Connect finishes.
	s.SetPendingSize(cols, rows)
	if s.session == nil {
		return fmt.Errorf("session not connected")
	}
	fmt.Printf("[Resize] %s -> cols=%d rows=%d\n", s.id, cols, rows)
	return s.session.WindowChange(rows, cols)
}

func (s *SSHSession) IsConnected() bool {
	return s.Status() == StatusConnected
}
