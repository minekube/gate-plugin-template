package ping

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	. "github.com/minekube/gate-plugin-template/util"
	"github.com/robinbraemer/event"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/proto/version"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is a ping plugin that handles ping events.
var Plugin = proxy.Plugin{
	Name: "Ping",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Hello from Ping plugin!")

		event.Subscribe(p.Event(), 0, onPing())

		return nil
	},
}

func onPing() func(*proxy.PingEvent) {
	line2 := &c.Text{
		Content: "Join, Test And Extend Your Gate Proxy!",
		S:       c.Style{Color: color.Gold, Bold: c.True},
	}
	return func(e *proxy.PingEvent) {
		clientVersion := version.Protocol(e.Connection().Protocol())
		line1 := &c.Text{
			Content: fmt.Sprintf("Hey %s user!\n", clientVersion),
			S:       c.Style{Color: color.Yellow},
		}

		p := e.Ping()
		p.Description = Join(line1, line2)
		p.Players.Max = p.Players.Online + 1
	}
}
