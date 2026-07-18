// Package mockmanager is a minimal, scriptable stand-in for the Gate-facing
// side of Manager (docs/protocol-gate-manager.md), for use in Go integration
// tests. It is intentionally independent from the managerclient package's
// types: messages are defined here from the wire protocol directly, so a
// test comparing the two catches real protocol drift, not just Go struct
// compatibility.
package mockmanager

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"testing"
)

// Message mirrors the NDJSON wire format of docs/protocol-gate-manager.md.
type Message struct {
	Type        string          `json:"type"`
	State       string          `json:"state,omitempty"`
	Running     string          `json:"running,omitempty"`
	Force       bool            `json:"force,omitempty"`
	RequestedBy string          `json:"requestedBy,omitempty"`
	Name        string          `json:"name,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Mode        string          `json:"mode,omitempty"`
	Events      json.RawMessage `json:"events,omitempty"`
	Entries     json.RawMessage `json:"entries,omitempty"`
}

// Server is a mock Manager listening for a single Gate connection at a time
// (reconnecting Gate clients get served by the same handler again).
type Server struct {
	Addr string

	t       testing.TB
	ln      net.Listener
	handler func(Message) []Message

	mu   sync.Mutex
	conn net.Conn
	recv []Message
}

// Start listens on 127.0.0.1:0 and, for every accepted connection, replies
// to each received message with whatever handler returns.
func Start(t testing.TB, handler func(Message) []Message) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mockmanager: listen: %v", err)
	}
	s := &Server{Addr: ln.Addr().String(), t: t, ln: ln, handler: handler}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conn = conn
		s.mu.Unlock()
		s.serve(conn)
	}
}

func (s *Server) serve(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		s.mu.Lock()
		s.recv = append(s.recv, msg)
		s.mu.Unlock()

		for _, reply := range s.handler(msg) {
			data, err := json.Marshal(reply)
			if err != nil {
				continue
			}
			data = append(data, '\n')
			if _, err := conn.Write(data); err != nil {
				return
			}
		}
	}
}

// Push writes msg to the currently connected client without waiting for an
// incoming message first (simulates unprompted pushes: evacuate-request,
// hardcore-ready).
func (s *Server) Push(msg Message) {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		s.t.Fatalf("mockmanager: Push called before a client connected")
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		s.t.Fatalf("mockmanager: marshal push message: %v", err)
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		s.t.Fatalf("mockmanager: push: %v", err)
	}
}

// CloseConn closes the current connection, simulating a Manager restart or
// network blip so tests can exercise Gate's reconnect logic.
func (s *Server) CloseConn() {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// Received returns a snapshot of every message received so far, in order.
func (s *Server) Received() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.recv))
	copy(out, s.recv)
	return out
}
