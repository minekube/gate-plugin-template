// Package permissions is a minimal, file-backed permission provider for Gate's
// builtin proxy commands (/server, /glist, /send).
//
// Gate ships no permission provider: a player's permission.Func is only ever set
// by whoever handles proxy.PermissionsSetupEvent, which is fired once per player
// at login. Until that happens the func is permission.Undefined, which evaluates
// to false - so setting `requireBuiltinCommandPermissions: true` on its own
// locks every builtin command for every player.
//
// This plugin answers that event from permissions.yml, so builtin commands can
// be granted per player (by UUID or username) without a permission plugin.
package permissions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/permission"
	"go.minekube.com/gate/pkg/util/uuid"
	"gopkg.in/yaml.v3"
)

// FileEnv names the environment variable that overrides the permission file
// path. Defaults to permissions.yml next to config.yml.
const FileEnv = "GATE_PERMISSIONS_FILE"

// DefaultFile is read when FileEnv is unset.
const DefaultFile = "permissions.yml"

// Plugin is registered on proxy.Plugins before gate.Execute().
var Plugin = proxy.Plugin{
	Name: "FilePermissions",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx).WithName("file-permissions")

		path := os.Getenv(FileEnv)
		if path == "" {
			path = DefaultFile
		}
		provider, err := LoadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			// Fail open on absence only: no file means "no opinion", which is
			// exactly Gate's default behaviour before any provider exists.
			log.Info("permission file not found, keeping Gate's default permissions",
				"file", path, "env", FileEnv)
			return nil
		}
		if err != nil {
			// Fail closed on a broken file: refuse to start rather than serve
			// with silently wrong permissions.
			return fmt.Errorf("permission file %q: %w", path, err)
		}

		// Priority 0: a later subscription (e.g. another permission plugin)
		// still wins via SetFunc, so this composes with other providers.
		event.Subscribe(p.Event(), 0, func(e *proxy.PermissionsSetupEvent) {
			player, ok := e.Subject().(proxy.Player)
			if !ok {
				return
			}
			e.SetFunc(provider.Func(player.ID(), player.Username()))
		})

		log.Info("loaded file permissions",
			"file", path,
			"players", len(provider.players),
			"defaultGrants", len(provider.defaultGrants),
			"denyEntries", len(provider.deny))
		return nil
	},
}

// Provider resolves a permission node to a TriState for one player.
type Provider struct {
	// defaultGrants apply to every player without an entry in players.
	defaultGrants []string
	// players maps a lower-cased player UUID or username to its grants.
	// A listed player replaces defaultGrants instead of inheriting them.
	players map[string][]string
	// deny maps a lower-cased player UUID or username to explicit denials.
	// Denials win over grants.
	deny map[string][]string
}

// fileConfig is the on-disk YAML shape.
type fileConfig struct {
	Default []string            `yaml:"default"`
	Players map[string][]string `yaml:"players"`
	Deny    map[string][]string `yaml:"deny"`
}

// LoadFile reads and validates a permission file.
func LoadFile(path string) (*Provider, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p, err := Parse(b)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Parse validates permission YAML.
func Parse(b []byte) (*Provider, error) {
	var cfg fileConfig
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // a typo must not silently grant nothing
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	p := &Provider{
		defaultGrants: cfg.Default,
		players:       make(map[string][]string, len(cfg.Players)),
		deny:          make(map[string][]string, len(cfg.Deny)),
	}
	if err := normalizeInto(p.players, cfg.Players, "players"); err != nil {
		return nil, err
	}
	if err := normalizeInto(p.deny, cfg.Deny, "deny"); err != nil {
		return nil, err
	}
	if len(p.defaultGrants) == 0 && len(p.players) == 0 {
		return nil, errors.New("no 'default' grants and no 'players' entries")
	}
	return p, nil
}

func normalizeInto(dst map[string][]string, src map[string][]string, field string) error {
	for rawKey, nodes := range src {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key == "" {
			return fmt.Errorf("%s: empty player key", field)
		}
		if _, dup := dst[key]; dup {
			return fmt.Errorf("%s: duplicate player key %q (UUIDs and names are case-insensitive)", field, rawKey)
		}
		dst[key] = nodes
	}
	return nil
}

// Func returns the permission.Func for one player.
//
// Semantics: an explicit denial returns False; a matching grant returns True;
// anything else returns Undefined so the decision stays open for other
// providers (and stays closed for requireBuiltinCommandPermissions, which
// treats Undefined as false).
func (p *Provider) Func(id uuid.UUID, username string) permission.Func {
	keys := playerKeys(id, username)

	grants := p.defaultGrants
	for _, k := range keys {
		if v, ok := p.players[k]; ok {
			grants = v
			break
		}
	}
	var denials []string
	for _, k := range keys {
		if v, ok := p.deny[k]; ok {
			denials = v
			break
		}
	}

	return func(node string) permission.TriState {
		if matchesAny(denials, node) {
			return permission.False
		}
		if matchesAny(grants, node) {
			return permission.True
		}
		return permission.Undefined
	}
}

// playerKeys returns the lower-cased lookup keys, UUID first so a UUID entry
// always beats a name entry of the same player.
func playerKeys(id uuid.UUID, username string) []string {
	keys := make([]string, 0, 2)
	if id != uuid.Nil {
		keys = append(keys, strings.ToLower(id.String()))
	}
	if name := strings.ToLower(strings.TrimSpace(username)); name != "" {
		keys = append(keys, name)
	}
	return keys
}

// matchesAny reports whether node is allowed by any pattern.
// Patterns are exact nodes, a trailing-wildcard prefix ("gate.command.*"),
// or a full wildcard ("*").
func matchesAny(patterns []string, node string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		switch {
		case pattern == "":
			continue
		case pattern == "*":
			return true
		case strings.HasSuffix(pattern, "*"):
			if strings.HasPrefix(node, strings.TrimSuffix(pattern, "*")) {
				return true
			}
		case pattern == node:
			return true
		}
	}
	return false
}
