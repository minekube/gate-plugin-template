package hardcoretogether

import (
	"context"
	"sync"
	"time"

	"go.minekube.com/gate/pkg/edition/java/proxy"
)

const transferConnectTimeout = 5 * time.Second

// onHardcoreReady handles Manager's hardcore-ready (docs/protocol-gate-manager.md
// 3.1a節): connect every player currently in lobby to hardcore, without
// requiring them to run /rta themselves (docs/specification.md 2.1節).
func (d *deps) onHardcoreReady(ctx context.Context) {
	hardcore := d.proxy.Server(d.hardcoreServer)
	if hardcore == nil {
		d.log.Error(nil, "hardcore server not registered", "server", d.hardcoreServer)
		return
	}

	var wg sync.WaitGroup
	for _, player := range d.proxy.Players() {
		cur := player.CurrentServer()
		if cur == nil || cur.Server().ServerInfo().Name() != d.lobbyServer {
			continue
		}
		wg.Add(1)
		go func(player proxy.Player) {
			defer wg.Done()
			connCtx, cancel := context.WithTimeout(ctx, transferConnectTimeout)
			defer cancel()
			player.CreateConnectionRequest(hardcore).ConnectWithIndication(connCtx)
		}(player)
	}
	wg.Wait()
}
