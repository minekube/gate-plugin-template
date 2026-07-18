package hardcoretogether

import (
	"fmt"

	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/permission"
	"go.minekube.com/gate/pkg/util/uuid"
)

const (
	// AdminPermission gates /start, /load (docs/architecture-gate.md 0.2節).
	AdminPermission = "hardcoretogether.admin"
	// serverCmdPermission is Gate's builtin /server permission node
	// (docs/architecture-gate.md 0.1節). Admins are granted this too so /server
	// becomes visible to them without a separate config.
	serverCmdPermission = "gate.command.server"
)

// parseAdmins converts the configured admin UUID strings into a lookup set.
func parseAdmins(raw []string) (map[uuid.UUID]struct{}, error) {
	admins := make(map[uuid.UUID]struct{}, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid admin UUID %q: %w", s, err)
		}
		admins[id] = struct{}{}
	}
	return admins, nil
}

// registerPermissions grants AdminPermission and serverCmdPermission to
// players listed in the admins set (docs/architecture-gate.md 0.2節).
func registerPermissions(p *proxy.Proxy, admins map[uuid.UUID]struct{}) {
	event.Subscribe(p.Event(), 0, func(e *proxy.PermissionsSetupEvent) {
		player, ok := e.Subject().(proxy.Player)
		if !ok {
			return
		}
		if _, isAdmin := admins[player.ID()]; !isAdmin {
			return
		}
		e.SetFunc(func(perm string) permission.TriState {
			switch perm {
			case AdminPermission, serverCmdPermission:
				return permission.True
			default:
				return permission.Undefined
			}
		})
	})
}

// requiresPermission builds a brigodier.RequireFn gating a command node on perm.
func requiresPermission(perm string) brigodier.RequireFn {
	return command.Requires(func(c *command.RequiresContext) bool {
		return c.Source.HasPermission(perm)
	})
}
