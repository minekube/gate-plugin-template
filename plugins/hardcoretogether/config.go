package hardcoretogether

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the hardcoreTogether section of Gate's config.yml (docs/architecture-gate.md 4節).
type Config struct {
	Admins         []string `yaml:"admins"`
	ManagerAddr    string   `yaml:"managerAddr"`
	LobbyServer    string   `yaml:"lobbyServer"`
	HardcoreServer string   `yaml:"hardcoreServer"`
}

type configFile struct {
	HardcoreTogether Config `yaml:"hardcoreTogether"`
}

// configPath returns the location of Gate's config.yml. It defaults to
// "config.yml" (Gate's own default) and can be overridden with
// HTG_CONFIG_PATH for setups that pass --config to Gate with a non-default
// path (docs/architecture-gate.md 6節, item 4).
func configPath() string {
	if p := os.Getenv("HTG_CONFIG_PATH"); p != "" {
		return p
	}
	return "config.yml"
}

// LoadConfig reads the hardcoreTogether section from Gate's config.yml.
func LoadConfig() (Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var f configFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	cfg := f.HardcoreTogether
	if cfg.ManagerAddr == "" {
		return Config{}, fmt.Errorf("hardcoreTogether.managerAddr is required in %s", path)
	}
	if cfg.LobbyServer == "" {
		return Config{}, fmt.Errorf("hardcoreTogether.lobbyServer is required in %s", path)
	}
	if cfg.HardcoreServer == "" {
		return Config{}, fmt.Errorf("hardcoreTogether.hardcoreServer is required in %s", path)
	}
	return cfg, nil
}
