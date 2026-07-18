package hardcoretogether

import (
	"context"

	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
)

// loadCommand implements /load <name>, /load <name> force, /load latest and
// /load latest force (docs/specification.md 2.1節). "latest" is just the string
// value of name; resolving it to the newest archive happens on Manager.
func loadCommand(d *deps) brigodier.LiteralNodeBuilder {
	run := func(force bool) brigodier.Command {
		return command.Command(func(ctx *command.Context) error {
			name := ctx.String("name")

			reqCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()

			result, err := d.client.Load(reqCtx, name, force, requesterName(ctx.Source))
			if err != nil {
				return ctx.Source.SendMessage(errorText("Managerと通信できません: " + err.Error()))
			}
			if result.Rejected {
				return ctx.Source.SendMessage(errorText(result.Reason))
			}
			return ctx.Source.SendMessage(infoText("アーカイブを復元しています..."))
		})
	}

	return brigodier.Literal("load").
		Requires(requiresPermission(AdminPermission)).
		Then(brigodier.Argument("name", brigodier.String).
			Executes(run(false)).
			Then(brigodier.Literal("force").Executes(run(true))))
}
