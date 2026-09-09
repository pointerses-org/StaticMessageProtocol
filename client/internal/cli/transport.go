package cli

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Transport is the interface for sending SMP messages.
type Transport interface {
	Send(msg []byte) ([]byte, error)
	Close() error
}

// ParseAddr extracts the IP from a smp@<ip> address.
func ParseAddr(addr string) (string, error) {
	if !strings.HasPrefix(addr, "smp@") {
		return "", fmt.Errorf("invalid address format: %s (expected smp@<ip>)", addr)
	}
	ip := strings.TrimPrefix(addr, "smp@")
	if ip == "" {
		return "", fmt.Errorf("empty IP address")
	}
	return ip, nil
}

// ParseUser extracts the username from a smp@<username> address.
func ParseUser(addr string) (string, error) {
	if !strings.HasPrefix(addr, "smp@") {
		return "", fmt.Errorf("invalid user format: %s (expected smp@<username>)", addr)
	}
	user := strings.TrimPrefix(addr, "smp@")
	if user == "" {
		return "", fmt.Errorf("empty username")
	}
	return user, nil
}

// SMPTransport is the native SMP protocol over TCP.
type SMPTransport struct {
	conn net.Conn
}

// NewSMPTransport connects to the server using native SMP protocol.
func NewSMPTransport(ip string, timeout int) (*SMPTransport, error) {
	addr := net.JoinHostPort(ip, "9932")
	if timeout <= 0 {
		timeout = 30
	}
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}
	return &SMPTransport{conn: conn}, nil
}

// Send sends a message and reads the response.
func (t *SMPTransport) Send(msg []byte) ([]byte, error) {
	t.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := t.conn.Write(msg); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	return readResponse(t.conn)
}

// Close closes the connection.
func (t *SMPTransport) Close() error {
	return t.conn.Close()
}

// TCPTransport is the SMP-over-TCP protocol (explicit TCP transport).
type TCPTransport struct {
	conn net.Conn
}

// NewTCPTransport connects using SMP-over-TCP.
func NewTCPTransport(ip string, timeout int) (*TCPTransport, error) {
	addr := net.JoinHostPort(ip, "9932")
	if timeout <= 0 {
		timeout = 30
	}
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeout)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}
	return &TCPTransport{conn: conn}, nil
}

// Send sends a message and reads the response.
func (t *TCPTransport) Send(msg []byte) ([]byte, error) {
	t.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := t.conn.Write(msg); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	return readResponse(t.conn)
}

// Close closes the connection.
func (t *TCPTransport) Close() error {
	return t.conn.Close()
}

// HTTPTransport is the SMP-over-HTTP protocol.
type HTTPTransport struct {
	client  *http.Client
	baseURL string
}

// NewHTTPTransport connects using SMP-over-HTTP.
func NewHTTPTransport(ip string, timeout int) (*HTTPTransport, error) {
	if timeout <= 0 {
		timeout = 30
	}
	return &HTTPTransport{
		client:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
		baseURL: fmt.Sprintf("http://%s:9932", ip),
	}, nil
}

// Send sends a message via HTTP POST and reads the response.
func (t *HTTPTransport) Send(msg []byte) ([]byte, error) {
	url := t.baseURL + "/smp"
	resp, err := t.client.Post(url, "application/x-smp", bytes.NewReader(msg))
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http read: %w", err)
	}
	return response, nil
}

// Close is a no-op for HTTP transport.
func (t *HTTPTransport) Close() error {
	return nil
}

// SSHTransport is the SMP-over-SSH protocol (stub).
type SSHTransport struct{}

// NewSSHTransport creates an SSH transport (not yet implemented).
func NewSSHTransport(ip string, timeout int) (*SSHTransport, error) {
	return nil, fmt.Errorf("SMP-over-SSH not yet implemented")
}

// Send sends a message via SSH (not yet implemented).
func (t *SSHTransport) Send(msg []byte) ([]byte, error) {
	return nil, fmt.Errorf("SMP-over-SSH not yet implemented")
}

// Close closes the SSH connection.
func (t *SSHTransport) Close() error {
	return nil
}

// Connect creates a transport based on the protocol string.
func Connect(protocol, ip string, timeout int) (Transport, error) {
	switch protocol {
	case "smp", "":
		return NewSMPTransport(ip, timeout)
	case "tcp":
		return NewTCPTransport(ip, timeout)
	case "http":
		return NewHTTPTransport(ip, timeout)
	case "ssh":
		return NewSSHTransport(ip, timeout)
	default:
		return nil, fmt.Errorf("unknown protocol: %s", protocol)
	}
}

// SelectProtocol returns the effective protocol from flags.
// If force is true, returns "http" (cannot be overridden).
func SelectProtocol(force bool, flags []string) string {
	if force {
		return "http"
	}
	for _, flag := range flags {
		switch flag {
		case "--http":
			return "http"
		case "--ssh":
			return "ssh"
		case "--tcp":
			return "tcp"
		case "--smp":
			return "smp"
		}
	}
	return "smp"
}

// readResponse reads a response from a TCP connection.
func readResponse(conn net.Conn) ([]byte, error) {
	var response []byte
	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			response = append(response, buf[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return response, fmt.Errorf("read: %w", err)
		}
		break
	}
	return response, nil
}
