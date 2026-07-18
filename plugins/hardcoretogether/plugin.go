// Package hardcoretogether implements the Gate-side commands and connection
// routing described in docs/specification.md and docs/architecture-gate.md. Manager (the
// process that actually manages the hardcore server, archives and records)
// is a separate project; this package only speaks the Gate⇔Manager protocol
// documented in docs/protocol-gate-manager.md.
package hardcoretogether

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"go.minekube.com/gate/pkg/edition/java/proxy"

	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/managerclient"
)

// Plugin registers the Hardcore Together Gate commands and Manager connection.
var Plugin = proxy.Plugin{
	Name: "HardcoreTogether",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx).WithName("hardcoretogether")

		cfg, err := LoadConfig()
		if err != nil {
			return fmt.Errorf("hardcoretogether: %w", err)
		}

		admins, err := parseAdmins(cfg.Admins)
		if err != nil {
			return fmt.Errorf("hardcoretogether: %w", err)
		}

		client := managerclient.New(cfg.ManagerAddr, log.WithName("managerclient"))

		d := &deps{
			proxy:          p,
			log:            log,
			client:         client,
			lobbyServer:    cfg.LobbyServer,
			hardcoreServer: cfg.HardcoreServer,
		}

		client.OnEvacuateRequest = d.onEvacuateRequest
		client.OnHardcoreReady = d.onHardcoreReady
		go client.Run(ctx)

		registerPermissions(p, admins)

		p.Command().Register(rtaCommand(d))
		p.Command().Register(lobbyCommand(d))
		p.Command().Register(startCommand(d))
		p.Command().Register(loadCommand(d))
		p.Command().Register(savedataCommand(d))
		p.Command().Register(senpanCommand(d))

		return nil
	},
}

// deps holds the dependencies shared by hardcoretogether's commands and
// Manager-callback handlers.
type deps struct {
	proxy  *proxy.Proxy
	log    logr.Logger
	client *managerclient.Client

	lobbyServer    string
	hardcoreServer string
}
