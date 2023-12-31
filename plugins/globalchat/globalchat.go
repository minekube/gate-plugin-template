package globalchat

import (
	"context"
	"github.com/go-logr/logr"
	. "github.com/minekube/gate-plugin-template/util"
	"github.com/robinbraemer/event"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is a global chat plugin that broadcasts a
// player's chat message to all players on the proxy.
var Plugin = proxy.Plugin{
	Name: "GlobalChat",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Hello from GlobalChat plugin!")

		pl := &plugin{proxy: p}
		event.Subscribe(p.Event(), 0, pl.onChat)

		return nil
	},
}

type plugin struct {
	proxy *proxy.Proxy
}

func (p *plugin) onChat(e *proxy.PlayerChatEvent) {
	// Another plugin may have already cancelled the event.
	if !e.Allowed() {
		return
	}

	// Cancel the event so that the message is not sent to the server of the player.
	e.SetAllowed(false)

	// Broadcast the message to all players.
	message := chatMessage(e.Player().Username(), e.Message())
	players := convertSlice[proxy.MessageSink](p.proxy.Players())
	proxy.BroadcastMessage(players, message)
}

func chatMessage(username, msg string) *c.Text {
	name := &c.Text{
		Content: username,
		S:       c.Style{Color: color.Gold},
	}
	separator := &c.Text{
		Content: ": ",
		S:       c.Style{Color: color.White},
	}
	message := &c.Text{
		Content: msg,
		S:       c.Style{Color: color.White, Italic: c.True},
	}
	return Join(name, separator, message)
}

func convertSlice[T any](a []proxy.Player) []T {
	b := make([]T, len(a))
	for i, v := range a {
		t, ok := v.(T)
		if ok {
			b[i] = t
		}
	}
	return b
}
