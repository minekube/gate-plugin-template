package managerclient_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"

	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/internal/mockmanager"
	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/managerclient"
)

func newClient(t *testing.T, addr string) *managerclient.Client {
	t.Helper()
	c := managerclient.New(addr, logr.Discard())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	waitConnected(t, c)
	return c
}

func waitConnected(t *testing.T, c *managerclient.Client) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Connected() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("client never connected to mock manager")
}

func TestQueryState(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "state-query" {
			t.Errorf("unexpected message type %q", msg.Type)
			return nil
		}
		return []mockmanager.Message{{Type: "state-response", State: "ready", Running: "true"}}
	})

	c := newClient(t, srv.Addr)

	state, running, err := c.QueryState(context.Background())
	if err != nil {
		t.Fatalf("QueryState: %v", err)
	}
	if state != managerclient.StateReady || running != managerclient.RunningTrue {
		t.Fatalf("got state=%q running=%q, want ready/true", state, running)
	}
}

func TestStartRejected(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "start" {
			return nil
		}
		return []mockmanager.Message{{Type: "start-rejected", Reason: "挑戦が進行中です"}}
	})

	c := newClient(t, srv.Addr)

	result, err := c.Start(context.Background(), false, "Steve")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !result.Rejected || result.Reason != "挑戦が進行中です" {
		t.Fatalf("got %+v, want rejected with reason", result)
	}

	recv := srv.Received()
	if len(recv) != 1 || recv[0].Force || recv[0].RequestedBy != "Steve" {
		t.Fatalf("manager received %+v, want single start{force:false,requestedBy:Steve}", recv)
	}
}

func TestStartAcceptedTriggersEvacuateHandshake(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "start" {
			return nil
		}
		return []mockmanager.Message{{Type: "evacuate-request", Reason: "force-reset"}}
	})

	c := newClient(t, srv.Addr)

	evacuated := make(chan string, 1)
	c.OnEvacuateRequest = func(_ context.Context, reason string) {
		evacuated <- reason
	}

	result, err := c.Start(context.Background(), true, "Steve")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if result.Rejected {
		t.Fatalf("got rejected result, want acceptance: %+v", result)
	}

	select {
	case reason := <-evacuated:
		if reason != "force-reset" {
			t.Fatalf("OnEvacuateRequest reason = %q, want force-reset", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("OnEvacuateRequest was not called")
	}

	// The client must send evacuate-complete on its own once the callback returns.
	deadline := time.Now().Add(time.Second)
	for {
		recv := srv.Received()
		if len(recv) >= 2 {
			if recv[1].Type != "evacuate-complete" {
				t.Fatalf("second message = %+v, want evacuate-complete", recv[1])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("manager never received evacuate-complete, got %+v", recv)
		}
		time.Sleep(5 * time.Millisecond)
	}

	recvFirst := srv.Received()[0]
	if !recvFirst.Force || recvFirst.RequestedBy != "Steve" {
		t.Fatalf("manager received %+v, want start{force:true,requestedBy:Steve}", recvFirst)
	}
}

func TestLoadRejected(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "load" {
			return nil
		}
		return []mockmanager.Message{{Type: "load-rejected", Reason: "アーカイブ save1 は存在しません"}}
	})

	c := newClient(t, srv.Addr)

	result, err := c.Load(context.Background(), "save1", false, "Steve")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !result.Rejected || result.Reason != "アーカイブ save1 は存在しません" {
		t.Fatalf("got %+v, want rejected with reason", result)
	}

	recv := srv.Received()
	if len(recv) != 1 || recv[0].Name != "save1" {
		t.Fatalf("manager received %+v, want single load{name:save1}", recv)
	}
}

func TestSaveData(t *testing.T) {
	// Example lifted verbatim from docs/protocol-gate-manager.md 3.6節.
	events := json.RawMessage(`[
		{"challengeId":"a1b2c3d4-...","type":"death","elapsedTime":900,"timestamp":"2026-07-18T12:05:00Z",
		 "deadPlayer":{"uuid":"11111111-2222-3333-4444-555555555555","name":"Steve"},"killLog":"Steve was slain by Zombie"}
	]`)

	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "savedata-query" {
			return nil
		}
		return []mockmanager.Message{{Type: "savedata-response", Events: events}}
	})

	c := newClient(t, srv.Addr)

	got, err := c.SaveData(context.Background())
	if err != nil {
		t.Fatalf("SaveData: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	e := got[0]
	if e.Type != "death" || e.ElapsedTime != 900 || e.DeadPlayer == nil || e.DeadPlayer.Name != "Steve" || e.KillLog != "Steve was slain by Zombie" {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestSenpan(t *testing.T) {
	// Example lifted verbatim from docs/protocol-gate-manager.md 3.7節.
	entries := json.RawMessage(`[
		{"player":{"uuid":"11111111-2222-3333-4444-555555555555","name":"Steve"},"count":3},
		{"player":{"uuid":"22222222-3333-4444-5555-666666666666","name":"Alex"},"count":1}
	]`)

	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "senpan-query" || msg.Mode != "count" {
			return nil
		}
		return []mockmanager.Message{{Type: "senpan-response", Mode: "count", Entries: entries}}
	})

	c := newClient(t, srv.Addr)

	got, err := c.Senpan(context.Background(), "count")
	if err != nil {
		t.Fatalf("Senpan: %v", err)
	}
	if len(got) != 2 || got[0].Player.Name != "Steve" || got[0].Count != 3 || got[1].Player.Name != "Alex" || got[1].Count != 1 {
		t.Fatalf("unexpected entries: %+v", got)
	}
}

func TestHardcoreReadyPush(t *testing.T) {
	srv := mockmanager.Start(t, func(mockmanager.Message) []mockmanager.Message { return nil })

	c := newClient(t, srv.Addr)

	ready := make(chan struct{}, 1)
	c.OnHardcoreReady = func(context.Context) { ready <- struct{}{} }

	srv.Push(mockmanager.Message{Type: "hardcore-ready"})

	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("OnHardcoreReady was not called")
	}
}

func TestReconnectAfterDisconnect(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "state-query" {
			return nil
		}
		return []mockmanager.Message{{Type: "state-response", State: "stopped", Running: "false"}}
	})

	c := newClient(t, srv.Addr)

	if _, _, err := c.QueryState(context.Background()); err != nil {
		t.Fatalf("QueryState before disconnect: %v", err)
	}

	srv.CloseConn()

	// Reconnecting can happen faster than this test can observe Connected()
	// dip to false in between, so just assert the client is usable again
	// shortly after the disconnect rather than racing to see the dip.
	waitConnected(t, c)

	// Connected() can flip true while still on the pre-CloseConn connection
	// for a brief moment (the client hasn't noticed the close yet), so a
	// query sent right away can land on a connection that's about to die
	// and never get a reply. Bound each attempt so that case fails fast and
	// gets retried against the connection that replaces it, rather than
	// blocking forever on context.Background().
	deadline := time.Now().Add(2 * time.Second)
	for {
		qctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		_, _, err := c.QueryState(qctx)
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("client never became usable again after reconnect: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestQueryStateWhenNotConnected(t *testing.T) {
	c := managerclient.New("127.0.0.1:1", logr.Discard()) // Run intentionally not started
	state, running, err := c.QueryState(context.Background())
	if err == nil {
		t.Fatal("expected an error when not connected")
	}
	if state != managerclient.StateUnknown || running != managerclient.RunningUnknown {
		t.Fatalf("got state=%q running=%q, want unknown/unknown", state, running)
	}
}
