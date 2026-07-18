package hardcoretogether

import (
	"context"

	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
)

// startCommand implements /start and /start force (docs/specification.md 2.1節).
// The running/exclusivity check itself happens on Manager; this only
// forwards the request and reports the outcome.
func startCommand(d *deps) brigodier.LiteralNodeBuilder {
	run := func(force bool) brigodier.Command {
		return command.Command(func(ctx *command.Context) error {
			reqCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()

			result, err := d.client.Start(reqCtx, force, requesterName(ctx.Source))
			if err != nil {
				return ctx.Source.SendMessage(errorText("Managerと通信できません: " + err.Error()))
			}
			if result.Rejected {
				return ctx.Source.SendMessage(errorText(result.Reason))
			}
			return ctx.Source.SendMessage(infoText("挑戦をリセットしています..."))
		})
	}

	return brigodier.Literal("start").
		Requires(requiresPermission(AdminPermission)).
		Executes(run(false)).
		Then(brigodier.Literal("force").Executes(run(true)))
}
