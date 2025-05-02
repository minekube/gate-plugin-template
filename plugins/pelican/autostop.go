package pelican

import (
	"context"
	"github.com/go-logr/logr"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"sync"
	"time"
)

var (
	lastConnectionMu sync.Mutex
	lastConnection   = make(map[string]time.Time)

	stopPlansMu sync.Mutex
	stopPlans   = make(map[string]context.CancelFunc)
)

// Called when a player connects to a server
func onConnectEvent(e *proxy.ServerPostConnectEvent) {
	serverName := e.Player().CurrentServer().Server().ServerInfo().Name()

	lastConnectionMu.Lock()
	lastConnection[serverName] = time.Now()
	lastConnectionMu.Unlock()

	// Cancel any pending stop for this server
	stopPlansMu.Lock()
	if cancel, ok := stopPlans[serverName]; ok {
		cancel()
		delete(stopPlans, serverName)
	}
	stopPlansMu.Unlock()
}

// Called when a player disconnects from a server
func planStop(cfg *Config, c *HttpClient, log logr.Logger, srv proxy.RegisteredServer) {
	serverName := srv.ServerInfo().Name()
	pelicanName, ok := cfg.Servers[serverName]
	if !ok {
		log.Info("No pelican server mapping for server", "server", serverName)
		return
	}

	delay := time.Duration(cfg.Delay) * time.Second

	// Only one planStop per server at a time
	stopPlansMu.Lock()
	if _, exists := stopPlans[serverName]; exists {
		stopPlansMu.Unlock()
		log.Info("A stop plan is already scheduled for this server", "server", serverName)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopPlans[serverName] = cancel
	stopPlansMu.Unlock()

	log.Info("Planning to stop server if still empty", "server", serverName, "pelican", pelicanName, "delay", delay)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		log.Info("Stop plan cancelled due to player join", "server", serverName)
		return
	case <-timer.C:
		// Continue
	}

	// Remove stop plan after timer
	stopPlansMu.Lock()
	delete(stopPlans, serverName)
	stopPlansMu.Unlock()

	if srv.Players().Len() == 0 {
		lastConnectionMu.Lock()
		last, ok := lastConnection[serverName]
		lastConnectionMu.Unlock()
		if ok && time.Since(last) < delay {
			log.Info("Players joined after delay, aborting stop", "server", serverName, "pelican", pelicanName)
			return
		}
		log.Info("No players joined, stopping server", "server", serverName, "pelican", pelicanName)
		err := c.StopServer(pelicanName)
		if err != nil {
			log.Error(err, "error stopping server", "server", pelicanName)
		}
	} else {
		log.Info("Players joined, aborting stop", "server", serverName, "pelican", pelicanName, "players", srv.Players().Len())
	}
}
