package hardcoretogether

import (
	"context"
	"sync"
	"time"

	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

const evacuateConnectTimeout = 5 * time.Second

// onEvacuateRequest handles Manager's evacuate-request (docs/protocol-gate-manager.md
// 3.5節): move every player currently connected to hardcore back to lobby
// before Manager stops the process (docs/specification.md 2.3節).
func (d *deps) onEvacuateRequest(ctx context.Context, reason string) {
	hardcore := d.proxy.Server(d.hardcoreServer)
	lobby := d.proxy.Server(d.lobbyServer)
	if hardcore == nil || lobby == nil {
		d.log.Error(nil, "lobby/hardcore server not registered", "lobby", d.lobbyServer, "hardcore", d.hardcoreServer)
		return
	}

	msg := evacuateMessage(reason)

	var wg sync.WaitGroup
	for _, player := range d.proxy.Players() {
		cur := player.CurrentServer()
		if cur == nil || cur.Server().ServerInfo().Name() != d.hardcoreServer {
			continue
		}
		wg.Add(1)
		go func(player proxy.Player) {
			defer wg.Done()
			_ = player.SendMessage(msg)
			connCtx, cancel := context.WithTimeout(ctx, evacuateConnectTimeout)
			defer cancel()
			player.CreateConnectionRequest(lobby).ConnectWithIndication(connCtx)
		}(player)
	}
	wg.Wait()
}

// evacuateMessage returns the notification wording specified in docs/specification.md 2.1節.
func evacuateMessage(reason string) c.Component {
	text := "ワールドリセットのためロビーに戻りました"
	if reason == "force-reset" {
		text = "管理者により強制リセットされました"
	}
	return infoText(text)
}
