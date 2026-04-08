// Package config loads and saves the user-level st8ctl configuration file.
//
// Default location: $XDG_CONFIG_HOME/st8ctl/config.yaml
// (typically ~/.config/st8ctl/config.yaml on Linux/macOS)
//
// Remote st8d example:
//
//	server:
//	  url: http://st8d.internal:8748
//	  token: my-secret-token
//
//	defaults:
//	  namespace: payments/prod
//	  branch: main
//
// Local instance example (url: local starts a temporary st8d):
//
//	server:
//	  url: local
//	  dir: ~/.st8
//
//	defaults:
//	  namespace: payments-dev
package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// LocalURL is the sentinel value for server.url that means "start a local st8d".
const LocalURL = "local"

// Config is the top-level configuration structure.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Defaults DefaultsConfig `yaml:"defaults"`
}

// ServerConfig holds connection settings for st8d.
//
// Set url to a remote address (e.g. "http://st8d.internal:8748") or to the
// special value "local" to start a temporary st8d on each command.
type ServerConfig struct {
	// URL is the st8d base URL, or "local" for a temporary local instance.
	URL string `yaml:"url,omitempty"`
	// Token is sent as "Authorization: Bearer <token>" with every request.
	// Must match the --token value configured on the st8d server.
	Token string `yaml:"token,omitempty"`
	// Dir is the data directory used when URL is "local".
	// Defaults to ~/.st8 when not set.
	Dir string `yaml:"dir,omitempty"`
	// Timeout is the HTTP client timeout for all requests.
	// Zero means no timeout.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// DefaultsConfig holds default scope values used when the corresponding CLI
// flags are not explicitly provided.
type DefaultsConfig struct {
	Namespace string `yaml:"namespace,omitempty"`
	Branch    string `yaml:"branch,omitempty"`
}

// DefaultPath returns the path to the default config file.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "st8ctl", "config.yaml"), nil
}

// Load reads and parses the config file at path. A missing file is not an
// error — an empty Config is returned instead.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes cfg as YAML to path, creating parent directories as needed.
// The file is written with mode 0600 and parent dirs with 0700.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// DefaultStateDir returns ~/.st8, falling back to .st8 if the home directory
// cannot be determined.
func DefaultStateDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".st8")
	}
	return ".st8"
}

// Or returns s if non-empty, otherwise fallback.
func Or(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
