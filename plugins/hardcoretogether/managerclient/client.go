// Package managerclient implements the Gate side of the Gate⇔Manager
// signal protocol documented in docs/protocol-gate-manager.md.
package managerclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// ErrNotConnected is returned by calls made while there is no active
// connection to Manager.
var ErrNotConnected = errors.New("managerclient: not connected to manager")

// State is hardcore's lifecycle state as tracked by Manager.
type State string

const (
	// StateUnknown is a Gate-local value meaning the state could not be
	// determined (not connected to Manager, or the query failed). It is
	// never sent by Manager itself.
	StateUnknown  State = "unknown"
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateReady    State = "ready"
)

// Running is hardcore's running flag as cached by Manager.
type Running string

const (
	RunningTrue    Running = "true"
	RunningFalse   Running = "false"
	RunningUnknown Running = "unknown"
)

// RecordEvent is one entry of a /savedata response (docs/protocol-gate-manager.md 3.6節).
type RecordEvent struct {
	ChallengeID string     `json:"challengeId"`
	Type        string     `json:"type"` // save | death | clear
	ElapsedTime int64      `json:"elapsedTime"`
	Timestamp   string     `json:"timestamp"`
	ArchiveName string     `json:"archiveName,omitempty"`
	Trigger     *Trigger   `json:"trigger,omitempty"`
	DeadPlayer  *PlayerRef `json:"deadPlayer,omitempty"`
	KillLog     string     `json:"killLog,omitempty"`
}

// Trigger identifies what caused a save/clear event.
type Trigger struct {
	Kind   string `json:"kind"` // boss | manual
	MobID  string `json:"mobId,omitempty"`
	Player string `json:"player,omitempty"`
}

// PlayerRef identifies a player by UUID and name.
type PlayerRef struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// SenpanEntry is one entry of a /senpan response (docs/protocol-gate-manager.md 3.7節).
type SenpanEntry struct {
	Player PlayerRef `json:"player"`
	Count  int       `json:"count"`
}

// CommandResult is the outcome of a Start or Load call.
type CommandResult struct {
	Rejected bool
	Reason   string
}

// message is the wire representation of every Gate⇔Manager NDJSON message.
// A single loosely-typed struct is used because the protocol is small
// (docs/protocol-gate-manager.md, ~12 message types) and every field maps
// 1:1 to a documented field of some message.
type message struct {
	Type string `json:"type"`

	// state-response
	State   string `json:"state,omitempty"`
	Running string `json:"running,omitempty"`

	// start / load
	Force       bool   `json:"force,omitempty"`
	RequestedBy string `json:"requestedBy,omitempty"`
	Name        string `json:"name,omitempty"`

	// start-rejected / load-rejected / evacuate-request
	Reason string `json:"reason,omitempty"`

	// savedata-response
	Events []RecordEvent `json:"events,omitempty"`

	// senpan-query / senpan-response
	Mode    string        `json:"mode,omitempty"`
	Entries []SenpanEntry `json:"entries,omitempty"`
}

// Client is a persistent connection to Manager. Create with New, then run
// Client.Run in a goroutine for the lifetime of the plugin.
type Client struct {
	addr string
	log  logr.Logger

	// OnEvacuateRequest is called synchronously for every evacuate-request
	// received (docs/protocol-gate-manager.md 3.5節). The client sends
	// evacuate-complete automatically once it returns.
	OnEvacuateRequest func(ctx context.Context, reason string)
	// OnHardcoreReady is called for every hardcore-ready notification
	// (docs/protocol-gate-manager.md 3.1a節).
	OnHardcoreReady func(ctx context.Context)

	connMu sync.Mutex
	conn   net.Conn

	writeMu sync.Mutex

	callMu sync.Mutex // serializes synchronous request/response round trips
	respMu sync.Mutex
	respCh chan message // set while a call() is outstanding
}

// New creates a Client that will connect to addr once Run is started.
func New(addr string, log logr.Logger) *Client {
	return &Client{addr: addr, log: log}
}

// Connected reports whether the TCP connection to Manager is currently up.
func (c *Client) Connected() bool {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn != nil
}

// Run connects to Manager and reconnects with backoff until ctx is done.
// It blocks; callers should run it in a goroutine.
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for ctx.Err() == nil {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.addr)
		if err != nil {
			c.log.Error(err, "failed to connect to manager, retrying", "backoff", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}

		backoff = time.Second
		c.log.Info("connected to manager")
		c.setConn(conn)

		c.readLoop(conn)

		c.setConn(nil)
		_ = conn.Close()
		c.log.Info("disconnected from manager, will reconnect")
	}
}

func (c *Client) setConn(conn net.Conn) {
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
}

func (c *Client) readLoop(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg message
		if err := json.Unmarshal(line, &msg); err != nil {
			c.log.Error(err, "failed to parse message from manager", "line", string(line))
			continue
		}
		c.dispatch(msg)
	}
}

// dispatch routes an incoming message either to a pending call() (the
// common case: it's a direct response to something Gate just sent) or to
// the push-message callbacks (evacuate-request, hardcore-ready).
func (c *Client) dispatch(msg message) {
	switch msg.Type {
	case "hardcore-ready":
		if c.OnHardcoreReady != nil {
			go c.OnHardcoreReady(context.Background())
		}
	case "evacuate-request":
		// evacuate-request doubles as the "accepted" outcome of a pending
		// start/load call (docs/protocol-gate-manager.md 4節), so deliver
		// it to any waiting call() in addition to handling it as a push.
		c.deliver(msg)
		go c.handleEvacuateRequest(msg)
	default:
		c.deliver(msg)
	}
}

func (c *Client) deliver(msg message) {
	c.respMu.Lock()
	ch := c.respCh
	c.respMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

func (c *Client) handleEvacuateRequest(msg message) {
	if c.OnEvacuateRequest != nil {
		c.OnEvacuateRequest(context.Background(), msg.Reason)
	}
	if err := c.send(message{Type: "evacuate-complete"}); err != nil {
		c.log.Error(err, "failed to send evacuate-complete")
	}
}

func (c *Client) send(msg message) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return ErrNotConnected
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = conn.Write(data)
	return err
}

// call sends req and waits for the next message Manager sends back on the
// connection. Calls are serialized (callMu) since the protocol has no
// request/response correlation ID (docs/protocol-gate-manager.md), which is
// sufficient because Gate never has two commands racing against Manager at
// once in this implementation.
func (c *Client) call(ctx context.Context, req message) (message, error) {
	if !c.Connected() {
		return message{}, ErrNotConnected
	}

	c.callMu.Lock()
	defer c.callMu.Unlock()

	ch := make(chan message, 1)
	c.respMu.Lock()
	c.respCh = ch
	c.respMu.Unlock()
	defer func() {
		c.respMu.Lock()
		if c.respCh == ch {
			c.respCh = nil
		}
		c.respMu.Unlock()
	}()

	if err := c.send(req); err != nil {
		return message{}, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return message{}, ctx.Err()
	}
}

// QueryState asks Manager for hardcore's current state (docs/protocol-gate-manager.md 3.1節).
func (c *Client) QueryState(ctx context.Context) (State, Running, error) {
	resp, err := c.call(ctx, message{Type: "state-query"})
	if err != nil {
		return StateUnknown, RunningUnknown, err
	}
	return State(resp.State), Running(resp.Running), nil
}

// Start sends a /start request (docs/protocol-gate-manager.md 3.2節).
func (c *Client) Start(ctx context.Context, force bool, requestedBy string) (CommandResult, error) {
	resp, err := c.call(ctx, message{Type: "start", Force: force, RequestedBy: requestedBy})
	if err != nil {
		return CommandResult{}, err
	}
	if resp.Type == "start-rejected" {
		return CommandResult{Rejected: true, Reason: resp.Reason}, nil
	}
	return CommandResult{}, nil
}

// Load sends a /load request (docs/protocol-gate-manager.md 3.3節). name may be "latest".
func (c *Client) Load(ctx context.Context, name string, force bool, requestedBy string) (CommandResult, error) {
	resp, err := c.call(ctx, message{Type: "load", Name: name, Force: force, RequestedBy: requestedBy})
	if err != nil {
		return CommandResult{}, err
	}
	if resp.Type == "load-rejected" {
		return CommandResult{Rejected: true, Reason: resp.Reason}, nil
	}
	return CommandResult{}, nil
}

// SaveData requests the /savedata listing (docs/protocol-gate-manager.md 3.6節).
func (c *Client) SaveData(ctx context.Context) ([]RecordEvent, error) {
	resp, err := c.call(ctx, message{Type: "savedata-query"})
	if err != nil {
		return nil, err
	}
	return resp.Events, nil
}

// Senpan requests the /senpan aggregation (docs/protocol-gate-manager.md 3.7節). mode is "list" or "count".
func (c *Client) Senpan(ctx context.Context, mode string) ([]SenpanEntry, error) {
	resp, err := c.call(ctx, message{Type: "senpan-query", Mode: mode})
	if err != nil {
		return nil, err
	}
	return resp.Entries, nil
}
