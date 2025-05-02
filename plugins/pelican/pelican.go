package pelican

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"go.minekube.com/common/minecraft/color"
	"go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"os"
	"time"
)

var wakeSent = make(map[string]time.Time)

var Plugin = proxy.Plugin{
	Name: "Pelican",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Pelican plugin loading...")

		v, err := initViper()
		if err != nil {
			return cli.Exit(err, 1)
		}

		cfg, err := LoadConfig(v)
		if err != nil {
			if !(errors.As(err, &viper.ConfigFileNotFoundError{}) || os.IsNotExist(err)) {
				err = fmt.Errorf("error reading config file %q: %w", v.ConfigFileUsed(), err)
				return cli.Exit(err, 2)
			}
		}

		c := NewHttpClient(cfg.Token, cfg.Url)

		event.Subscribe(p.Event(), 0, onKickedFromServerEvent(log, cfg, c))
		if cfg.Autostop {
			event.Subscribe(p.Event(), 0, onDisconnectEvent(log, cfg, c))
			event.Subscribe(p.Event(), 0, onConnectEvent)
		}

		log.Info("servers configured", "count", len(cfg.Servers))
		log.Info("Pelican plugin loaded.")

		return nil
	},
}

func onKickedFromServerEvent(log logr.Logger, cfg *Config, c *HttpClient) func(*proxy.KickedFromServerEvent) {
	return func(e *proxy.KickedFromServerEvent) {
		if e.OriginalReason() != nil {
			return
		}

		if s, ok := cfg.Servers[e.Server().ServerInfo().Name()]; ok {
			if _, ok := wakeSent[s]; ok {
				if time.Since(wakeSent[s]) < 30*time.Second {
					log.Info("Already sent wake to Pelican", "server", e.Server().ServerInfo().Name(), "pelican", s)
					result := &proxy.RedirectPlayerKickResult{Message: &component.Text{
						Content: "Server is starting, please wait...",
						S: component.Style{
							Color: color.Yellow,
						},
					}}
					e.SetResult(result)
					return
				} else {
					delete(wakeSent, s)
				}
			}

			log.Info("Sending wake to Pelican", "server", e.Server().ServerInfo().Name(), "pelican", s)

			err := c.StartServer(s)
			if err != nil {
				log.Error(err, "error starting server", "server", s)
				result := &proxy.RedirectPlayerKickResult{Message: &component.Text{
					Content: "Error starting server",
					S: component.Style{
						Color: color.Red,
					},
				}}
				e.SetResult(result)
				return
			}

			result := &proxy.RedirectPlayerKickResult{Message: &component.Text{
				Content: "Starting server...",
				S: component.Style{
					Color: color.Yellow,
				},
			}}
			e.SetResult(result)
			wakeSent[s] = time.Now()
		}
	}
}

func onDisconnectEvent(log logr.Logger, cfg *Config, c *HttpClient) func(*proxy.DisconnectEvent) {
	return func(e *proxy.DisconnectEvent) {
		if conn := e.Player().CurrentServer(); conn != nil {
			srv := conn.Server()
			if s, ok := cfg.Servers[srv.ServerInfo().Name()]; ok {
				if srv.Players().Len() == 0 {
					log.Info("Planning to stop the server", "server", srv.ServerInfo().Name(), "pelican", s)
					go planStop(cfg, c, log, srv)
				}
			}
		}
	}
}
