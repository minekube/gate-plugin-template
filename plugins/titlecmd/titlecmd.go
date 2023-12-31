package titlecmd

import (
	"context"
	"github.com/go-logr/logr"
	. "github.com/minekube/gate-plugin-template/util"
	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/edition/java/title"
	"time"
)

// Plugin is a ping plugin that handles ping events.
var Plugin = proxy.Plugin{
	Name: "TitleCmd",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Hello from Ping plugin!")

		p.Command().Register(titleCommand())
		event.Subscribe(p.Event(), 0, func(e *proxy.ServerConnectedEvent) {
			go func() {
				time.Sleep(time.Second)
				sendUsage(e.Player())
			}()
		})

		return nil
	},
}

func sendUsage(player proxy.Player) {
	usage := &c.Text{
		Content: "Try out: /title <title> [subtitle]\n",
	}
	const sampleCmd = `"/title "&5Hello" "&6World"`
	example := &c.Text{
		Content: "Example: " + sampleCmd,
		S: c.Style{
			ClickEvent: c.NewClickEvent(c.SuggestCommandAction, sampleCmd),
			HoverEvent: c.NewHoverEvent(c.ShowTextAction, &c.Text{Content: sampleCmd}),
		},
	}

	_ = player.SendMessage(Join(usage, example))
}

func titleCommand() brigodier.LiteralNodeBuilder {
	showTitle := command.Command(func(ctx *command.Context) error {
		player, ok := ctx.Source.(proxy.Player)
		if !ok {
			return ctx.Source.SendMessage(&c.Text{Content: "You must be a player to run this command."})
		}

		return title.ShowTitle(player, &title.Options{
			Title:    Text(ctx.String("title")),
			Subtitle: Text(ctx.String("subtitle")), // empty if arg not provided
		})
	})

	return brigodier.Literal("title").
		Then(brigodier.Argument("title", brigodier.String).Executes(showTitle).
			Then(brigodier.Argument("subtitle", brigodier.StringPhrase).Executes(showTitle)))
}
