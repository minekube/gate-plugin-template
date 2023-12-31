package bossbar

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/bossbar"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"time"
)

// Plugin is a boss bar plugin that displays a boss bar to the player upon login.
var Plugin = proxy.Plugin{
	Name: "BossBar",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Hello from BossBar plugin!")

		event.Subscribe(p.Event(), 0, bossBarDisplay())

		return nil
	},
}

func bossBarDisplay() func(*proxy.PostLoginEvent) {
	// Create shared boss bar for all players
	sharedBar := bossbar.New(
		&c.Text{Content: "Welcome to Gate Sample proxy!", S: c.Style{
			Color: color.White,
			Bold:  c.True,
		}},
		1,
		bossbar.BlueColor,
		bossbar.ProgressOverlay,
	)

	updateBossBar := func(bar bossbar.BossBar, player proxy.Player) {
		now := time.Now()
		text := &c.Text{Extra: []c.Component{
			&c.Text{
				Content: fmt.Sprintf("Hello %s! ", player.Username()),
				S:       c.Style{Color: color.Yellow},
			},
			&c.Text{
				Content: fmt.Sprintf("It's %s", now.Format("15:04:05 PM")),
				S:       c.Style{Color: color.Gold},
			},
		}}
		bar.SetName(text)
		bar.SetPercent(float32(now.Second()) / 60)
	}

	return func(e *proxy.PostLoginEvent) {
		player := e.Player()

		// Add player to shared boss bar
		_ = sharedBar.AddViewer(player)

		// Create own boss bar for player
		playerBar := bossbar.New(
			&c.Text{},
			bossbar.MinProgress,
			bossbar.RedColor,
			bossbar.ProgressOverlay,
		)
		// Show it to player
		_ = playerBar.AddViewer(player)

		// Update boss bars every second until player disconnects.
		// Run in new goroutine to unblock login event handler!
		go tick(player.Context(), time.Second, func() {
			updateBossBar(playerBar, player)
		})
	}
}

// tick runs a function every interval until the context is cancelled.
func tick(ctx context.Context, interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			fn()
		case <-ctx.Done():
			return
		}
	}
}
