package pelican

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"go.minekube.com/gate/pkg/gate"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"strings"
)

type Config struct {
	Token   string            `yaml:"token" json:"token"`
	Url     string            `yaml:"url" json:"url"`
	Servers map[string]string `yaml:"servers,omitempty" json:"servers,omitempty"`
}

var DefaultConfig = Config{
	Token: "Your Pelican token",
	Url:   "https://demo.pelican.dev",
	Servers: map[string]string{
		"server1": "The UUID of the server you want to connect to",
	},
}

// LoadConfig loads in config.PelicanConfig from viper.
// It is used by Start with the packages Viper if no WithConfig option is given.
func LoadConfig(v *viper.Viper) (*Config, error) {
	// Clone default config
	cfg := func() Config { return DefaultConfig }()
	// Load in Gate config
	if err := fixedReadInConfig(v, &cfg); err != nil {
		return &cfg, fmt.Errorf("error loading config: %w", err)
	}
	return &cfg, nil
}

func initViper() (*viper.Viper, error) {
	v := gate.Viper
	v.SetConfigName("pelican")
	v.AddConfigPath(".")
	// Load Environment Variables
	v.SetEnvPrefix("GATE_PELICAN")
	v.AutomaticEnv() // read in environment variables that match
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return v, nil
}

func fixedReadInConfig(v *viper.Viper, defaultConfig *Config) error {
	if defaultConfig == nil {
		return v.ReadInConfig()
	}

	configFile := v.ConfigFileUsed()
	if configFile == "" {
		// Try to find config file using Viper's config finder logic
		if err := v.ReadInConfig(); err != nil {
			return err
		}
		configFile = v.ConfigFileUsed()
		if configFile == "" {
			return nil // no config file found
		}
	}

	var (
		unmarshal func([]byte, any) error
		marshal   func(any) ([]byte, error)
	)
	switch path.Ext(configFile) {
	case ".yaml", ".yml":
		unmarshal = yaml.Unmarshal
		marshal = yaml.Marshal
	case ".json":
		unmarshal = json.Unmarshal
		marshal = json.Marshal
	default:
		return fmt.Errorf("unsupported config file format %q", configFile)
	}
	b, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading config file %q: %w", configFile, err)
	}
	if err = unmarshal(b, defaultConfig); err != nil {
		return fmt.Errorf("error unmarshaling config file %q to %T: %w", configFile, defaultConfig, err)
	}
	if b, err = marshal(defaultConfig); err != nil {
		return fmt.Errorf("error marshaling config file %q: %w", configFile, err)
	}

	return v.ReadConfig(bytes.NewReader(b))
}
