package hardcoretogether

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/util/permission"

	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/internal/mockmanager"
	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/managerclient"
)

// fakeSource is a minimal command.Source for driving commands in tests
// without a real Minecraft client connection.
type fakeSource struct {
	allowed bool

	mu       sync.Mutex
	messages []string
}

func (f *fakeSource) HasPermission(string) bool { return f.allowed }

func (f *fakeSource) PermissionValue(string) permission.TriState {
	if f.allowed {
		return permission.True
	}
	return permission.Undefined
}

func (f *fakeSource) SendMessage(msg c.Component, _ ...command.MessageOption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if text, ok := msg.(*c.Text); ok {
		f.messages = append(f.messages, text.Content)
	}
	return nil
}

func (f *fakeSource) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) == 0 {
		return ""
	}
	return f.messages[len(f.messages)-1]
}

// newTestDeps connects a managerclient.Client to srv and wraps it in deps
// with no *proxy.Proxy. This is enough to exercise /start, /load,
// /savedata and /senpan, which never touch d.proxy directly; /rta, /lobby
// and the evacuate/transfer callbacks need a real player connection and are
// out of scope for this test tier.
func newTestDeps(t *testing.T, srv *mockmanager.Server) *deps {
	t.Helper()
	client := managerclient.New(srv.Addr, logr.Discard())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go client.Run(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for !client.Connected() {
		if time.Now().After(deadline) {
			t.Fatal("client never connected to mock manager")
		}
		time.Sleep(5 * time.Millisecond)
	}

	return &deps{client: client, log: logr.Discard()}
}

func newTestManager(d *deps) *command.Manager {
	mgr := &command.Manager{}
	mgr.Register(startCommand(d))
	mgr.Register(loadCommand(d))
	mgr.Register(savedataCommand(d))
	mgr.Register(senpanCommand(d))
	return mgr
}

func TestStartCommand_Rejected(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "start" {
			return nil
		}
		return []mockmanager.Message{{Type: "start-rejected", Reason: "挑戦が進行中です"}}
	})
	d := newTestDeps(t, srv)
	mgr := newTestManager(d)
	src := &fakeSource{allowed: true}

	if err := mgr.Do(context.Background(), src, "start"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if src.last() != "挑戦が進行中です" {
		t.Fatalf("last message = %q, want rejection reason", src.last())
	}
}

func TestStartCommand_ForceAccepted(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "start" {
			return nil
		}
		if !msg.Force {
			t.Errorf("start message had force=false, want true for /start force")
		}
		return []mockmanager.Message{{Type: "evacuate-request", Reason: "force-reset"}}
	})
	d := newTestDeps(t, srv)
	mgr := newTestManager(d)
	src := &fakeSource{allowed: true}

	if err := mgr.Do(context.Background(), src, "start force"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(src.last(), "リセット") {
		t.Fatalf("last message = %q, want an acceptance notice", src.last())
	}
}

func TestStartCommand_RequiresPermission(t *testing.T) {
	srv := mockmanager.Start(t, func(mockmanager.Message) []mockmanager.Message { return nil })
	d := newTestDeps(t, srv)
	mgr := newTestManager(d)
	src := &fakeSource{allowed: false}

	if err := mgr.Do(context.Background(), src, "start"); err == nil {
		t.Fatal("expected an error for an unprivileged source, got none")
	}
	if recv := srv.Received(); len(recv) != 0 {
		t.Fatalf("manager received %+v, want nothing (command should have been blocked)", recv)
	}
}

func TestLoadCommand(t *testing.T) {
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "load" {
			return nil
		}
		if msg.Name != "latest" {
			t.Errorf("load message name = %q, want latest", msg.Name)
		}
		return []mockmanager.Message{{Type: "load-rejected", Reason: "アーカイブ latest は存在しません"}}
	})
	d := newTestDeps(t, srv)
	mgr := newTestManager(d)
	src := &fakeSource{allowed: true}

	if err := mgr.Do(context.Background(), src, "load latest"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if src.last() != "アーカイブ latest は存在しません" {
		t.Fatalf("last message = %q, want rejection reason", src.last())
	}
}

func TestSavedataCommand(t *testing.T) {
	events := json.RawMessage(`[
		{"challengeId":"a1b2c3d4-...","type":"clear","elapsedTime":1500,"timestamp":"2026-07-18T12:45:00Z",
		 "trigger":{"kind":"boss","mobId":"twilightforest:ur_ghast"}}
	]`)
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "savedata-query" {
			return nil
		}
		return []mockmanager.Message{{Type: "savedata-response", Events: events}}
	})
	d := newTestDeps(t, srv)
	mgr := newTestManager(d)
	src := &fakeSource{allowed: true}

	if err := mgr.Do(context.Background(), src, "savedata"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(src.last(), "a1b2c3d4-...") || !strings.Contains(src.last(), "クリア") {
		t.Fatalf("last message = %q, want it to mention the challengeId and clear event", src.last())
	}
}

func TestSenpanCommand(t *testing.T) {
	entries := json.RawMessage(`[{"player":{"uuid":"11111111-2222-3333-4444-555555555555","name":"Steve"},"count":3}]`)
	srv := mockmanager.Start(t, func(msg mockmanager.Message) []mockmanager.Message {
		if msg.Type != "senpan-query" || msg.Mode != "list" {
			return nil
		}
		return []mockmanager.Message{{Type: "senpan-response", Mode: "list", Entries: entries}}
	})
	d := newTestDeps(t, srv)
	mgr := newTestManager(d)
	src := &fakeSource{allowed: true}

	if err := mgr.Do(context.Background(), src, "senpan list"); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(src.last(), "Steve") || !strings.Contains(src.last(), "3回") {
		t.Fatalf("last message = %q, want it to mention Steve and 3回", src.last())
	}
}
