package ping

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	. "github.com/minekube/gate-plugin-template/util"
	"github.com/minekube/gate-plugin-template/util/mini"
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
	line2 := mini.Gradient(
		"Join, test and extend your Gate proxy!",
		c.Style{Bold: c.True},
		*color.Yellow.RGB, *color.Gold.RGB, *color.Red.RGB,
	)

	return func(e *proxy.PingEvent) {
		clientVersion := version.Protocol(e.Connection().Protocol())
		line1 := mini.Gradient(
			fmt.Sprintf("Hey %s user!\n", clientVersion),
			c.Style{},
			*color.White.RGB, *color.LightPurple.RGB,
		)

		p := e.Ping()
		p.Description = Join(line1, line2)
		p.Players.Max = p.Players.Online + 1
	}
}
