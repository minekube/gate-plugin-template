package tablist

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is a demo TabList plugin that sets a
// custom header and footer when a player logs in.
var Plugin = proxy.Plugin{
	Name: "TabList",
	Init: func(ctx context.Context, p *proxy.Proxy) error {

		// Get the logger for this plugin.
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Hello from TabList plugin!")

		// Register some event handlers.
		event.Subscribe(p.Event(), 0, onPostLogin)

		return nil
	},
}

// onPostLogin is called when a player successfully logged in.
func onPostLogin(e *proxy.PostLoginEvent) {
	// Make use of the Text library.
	header := &c.Text{
		Content: fmt.Sprintf("\nWelcome %s on my network!\n", e.Player().Username()),
		S:       c.Style{Color: color.Yellow, Bold: c.True},
	}
	footer := &c.Text{
		Content: "\nThis is a demo TabList plugin.\n",
		S:       c.Style{Color: color.White, Italic: c.True},
	}

	// Most Gate methods are thread-safe and can be called from any goroutine.
	// We could also handle errors gracefully, like the tab list could not be sent
	// to the player because they disconnected, but we can often ignore them for simplicity.
	_ = e.Player().TabList().SetHeaderFooter(header, footer)
}
