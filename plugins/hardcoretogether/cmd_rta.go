package hardcoretogether

import (
	"context"
	"time"

	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"

	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/managerclient"
)

const commandTimeout = 10 * time.Second

// rtaCommand implements /rta (docs/specification.md 2.1節): only connects while
// hardcore is Ready; otherwise reports the current state instead of
// attempting a connection (fixes the old LoginControl bug of not checking
// liveness first).
func rtaCommand(d *deps) brigodier.LiteralNodeBuilder {
	return brigodier.Literal("rta").Executes(command.Command(func(ctx *command.Context) error {
		player, ok := ctx.Source.(proxy.Player)
		if !ok {
			return ctx.Source.SendMessage(errorText("プレイヤーのみ実行できます"))
		}

		qctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()
		state, _, err := d.client.QueryState(qctx)
		if err != nil {
			return ctx.Source.SendMessage(errorText("Managerと通信できません"))
		}

		switch state {
		case managerclient.StateReady:
			hardcore := d.proxy.Server(d.hardcoreServer)
			if hardcore == nil {
				return ctx.Source.SendMessage(errorText("hardcoreサーバーが登録されていません"))
			}
			connCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()
			player.CreateConnectionRequest(hardcore).ConnectWithIndication(connCtx)
			return nil
		case managerclient.StateStarting:
			return ctx.Source.SendMessage(infoText("起動処理中です。しばらくお待ちください"))
		default:
			return ctx.Source.SendMessage(infoText("hardcoreサーバーは停止中です"))
		}
	}))
}

// lobbyCommand implements /lobby (docs/specification.md 2.1節): always available.
func lobbyCommand(d *deps) brigodier.LiteralNodeBuilder {
	return brigodier.Literal("lobby").Executes(command.Command(func(ctx *command.Context) error {
		player, ok := ctx.Source.(proxy.Player)
		if !ok {
			return ctx.Source.SendMessage(errorText("プレイヤーのみ実行できます"))
		}
		lobby := d.proxy.Server(d.lobbyServer)
		if lobby == nil {
			return ctx.Source.SendMessage(errorText("lobbyサーバーが登録されていません"))
		}
		connCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()
		player.CreateConnectionRequest(lobby).ConnectWithIndication(connCtx)
		return nil
	}))
}
