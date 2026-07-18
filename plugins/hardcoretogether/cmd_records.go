package hardcoretogether

import (
	"context"
	"fmt"
	"strings"

	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"

	"github.com/minekube/gate-plugin-template/plugins/hardcoretogether/managerclient"
)

// savedataCommand implements /savedata (docs/specification.md 2.2節). Manager
// already sorts events by timestamp (docs/protocol-gate-manager.md 3.6節);
// this only formats them for display. The exact display format is an open
// item (docs/architecture-gate.md 6節).
func savedataCommand(d *deps) brigodier.LiteralNodeBuilder {
	return brigodier.Literal("savedata").Executes(command.Command(func(ctx *command.Context) error {
		reqCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()

		events, err := d.client.SaveData(reqCtx)
		if err != nil {
			return ctx.Source.SendMessage(errorText("Managerと通信できません: " + err.Error()))
		}
		if len(events) == 0 {
			return ctx.Source.SendMessage(infoText("挑戦記録はまだありません"))
		}

		var b strings.Builder
		for _, e := range events {
			b.WriteString(formatRecordEvent(e))
			b.WriteString("\n")
		}
		return ctx.Source.SendMessage(infoText(b.String()))
	}))
}

func formatRecordEvent(e managerclient.RecordEvent) string {
	switch e.Type {
	case "death":
		name := "?"
		if e.DeadPlayer != nil {
			name = e.DeadPlayer.Name
		}
		return fmt.Sprintf("[%s] %s 死亡 (%s) elapsed=%ds", e.ChallengeID, name, e.KillLog, e.ElapsedTime)
	case "clear":
		return fmt.Sprintf("[%s] クリア elapsed=%ds", e.ChallengeID, e.ElapsedTime)
	case "save":
		return fmt.Sprintf("[%s] セーブ: %s elapsed=%ds", e.ChallengeID, e.ArchiveName, e.ElapsedTime)
	default:
		return fmt.Sprintf("[%s] %s", e.ChallengeID, e.Type)
	}
}

// senpanCommand implements /senpan list|count (docs/specification.md 2.2節).
func senpanCommand(d *deps) brigodier.LiteralNodeBuilder {
	run := func(mode string) brigodier.Command {
		return command.Command(func(ctx *command.Context) error {
			reqCtx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()

			entries, err := d.client.Senpan(reqCtx, mode)
			if err != nil {
				return ctx.Source.SendMessage(errorText("Managerと通信できません: " + err.Error()))
			}
			if len(entries) == 0 {
				return ctx.Source.SendMessage(infoText("戦犯はいません"))
			}

			var b strings.Builder
			for _, e := range entries {
				fmt.Fprintf(&b, "%s: %d回\n", e.Player.Name, e.Count)
			}
			return ctx.Source.SendMessage(infoText(b.String()))
		})
	}

	return brigodier.Literal("senpan").
		Then(brigodier.Literal("list").Executes(run("list"))).
		Then(brigodier.Literal("count").Executes(run("count")))
}
