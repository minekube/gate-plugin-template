package hardcoretogether

import (
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func errorText(s string) c.Component {
	return &c.Text{Content: s, S: c.Style{Color: color.Red}}
}

func infoText(s string) c.Component {
	return &c.Text{Content: s, S: c.Style{Color: color.Yellow}}
}

// requesterName returns the invoking player's username, or "console" for
// non-player command sources.
func requesterName(src command.Source) string {
	if player, ok := src.(proxy.Player); ok {
		return player.Username()
	}
	return "console"
}
