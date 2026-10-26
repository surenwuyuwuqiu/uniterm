package session

import (
	"errors"
	"golang.org/x/crypto/ssh"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestSFTPStartupErrorClassification(t *testing.T) {
	cases := []struct{ name, message, code string }{
		{"missing server EOF", "error receiving version packet from server: server unexpectedly closed connection: unexpected EOF", "sftp_unavailable"},
		{"missing server before init", "error sending init packet to server: failed to send packet: EOF", "sftp_unavailable"},
		{"rejected subsystem", "ssh: subsystem request failed", "sftp_unavailable"},
		{"polluted stream", "received packet too long: 12345", "sftp_subsystem_polluted"},
		{"timeout", "error receiving version packet from server: i/o timeout", ""},
		{"reset", "error receiving version packet from server: connection reset by peer", ""},
		{"auth failure", "ssh: handshake failed: unable to authenticate", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := errors.New(tc.message)
			got := hintSFTPStartupError(original, errors.New("fallback failed")).Error()
			if tc.code == "" {
				if strings.Contains(got, "unitermerr:") {
					t.Fatalf("unexpected recovery code: %s", got)
				}
			} else if !strings.HasPrefix(got, "unitermerr:"+tc.code+"|") || !strings.Contains(got, tc.message) {
				t.Fatalf("lost classification or diagnostic: %s", got)
			}
			if tc.code == "sftp_unavailable" && isSFTPStreamPolluted(original) {
				t.Fatal("unavailable server misclassified as stdout pollution")
			}
		})
	}
}

// Emulate a device that accepts SSH but has no working SFTP subsystem. SCP
// capability failures are separate from SSH login and must fail Connect.
func startFileStartupServer(t *testing.T, scpAvailable bool) ConnectionConfig {
	t.Helper()
	signer, err := ssh.ParsePrivateKey([]byte(testHostKeyPEM))
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		server, channels, requests, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			channel, requests, err := incoming.Accept()
			if err != nil {
				continue
			}
			go func() {
				defer channel.Close()
				for req := range requests {
					switch req.Type {
					case "subsystem":
						// Accept then terminate, like a subsystem whose executable is missing.
						_ = req.Reply(true, nil)
						return
					case "exec":
						var payload struct{ Command string }
						if ssh.Unmarshal(req.Payload, &payload) != nil {
							_ = req.Reply(false, nil)
							return
						}
						_ = req.Reply(true, nil)
						status := uint32(127)
						if payload.Command == "pwd" {
							_, _ = channel.Write([]byte("/home/test\n"))
							status = 0
						} else if payload.Command == scpCapabilityCommand && scpAvailable {
							status = 0
						}
						_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
						return
					default:
						_ = req.Reply(false, nil)
					}
				}
			}()
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	return ConnectionConfig{Host: host, Port: port, User: "test", AuthType: "password", Password: "test"}
}

func TestSFTPStartupMissingServer(t *testing.T) {
	config := startFileStartupServer(t, false)
	s := NewSFTPSession("missing-sftp-server")
	err := s.Connect(config)
	if err == nil {
		_ = s.Disconnect()
		t.Fatal("expected SFTP startup failure")
	}
	if !strings.Contains(err.Error(), "unitermerr:sftp_unavailable|") {
		t.Fatalf("missing server must offer SCP without blaming login scripts: %v", err)
	}
	if s.Status() != StatusError {
		t.Fatalf("status: %v", s.Status())
	}
}

func TestSCPStartupCapabilities(t *testing.T) {
	for _, available := range []bool{false, true} {
		t.Run(strconv.FormatBool(available), func(t *testing.T) {
			config := startFileStartupServer(t, available)
			s := NewSCPSession("scp-capability-test")
			defer s.Disconnect()
			err := s.Connect(config)
			if available {
				if err != nil {
					t.Fatal(err)
				}
				if !s.IsConnected() {
					t.Fatal("expected connected SCP session")
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "unitermerr:scp_unavailable|") {
					t.Fatalf("SSH login must not count as SCP support: %v", err)
				}
				if s.IsConnected() {
					t.Fatal("unsupported SCP session reported connected")
				}
			}
		})
	}
}
