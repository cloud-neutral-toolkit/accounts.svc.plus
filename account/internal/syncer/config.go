package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config defines how account snapshots should be synchronised between
// environments.
type Config struct {
	Local  LocalConfig  `yaml:"local"`
	Remote RemoteConfig `yaml:"remote"`
}

// LocalConfig describes the local database connection and snapshot options.
type LocalConfig struct {
	DSN          string        `yaml:"dsn"`
	EmailKeyword string        `yaml:"email_keyword"`
	ExportPath   string        `yaml:"export_path"`
	Import       ImportOptions `yaml:"import"`
}

// ImportOptions configures how imported snapshots should be reconciled with the
// target database.
type ImportOptions struct {
	Merge         bool     `yaml:"merge"`
	MergeStrategy string   `yaml:"merge_strategy"`
	DryRun        bool     `yaml:"dry_run"`
	Allowlist     []string `yaml:"allowlist"`
}

// RemoteConfig contains SSH connection details and remote runtime options.
type RemoteConfig struct {
	Address        string            `yaml:"address"`
	Port           int               `yaml:"port"`
	User           string            `yaml:"user"`
	IdentityFile   string            `yaml:"identity_file"`
	KnownHostsFile string            `yaml:"known_hosts_file"`
	AccountDir     string            `yaml:"account_dir"`
	ExportPath     string            `yaml:"export_path"`
	ImportPath     string            `yaml:"import_path"`
	Env            map[string]string `yaml:"env"`
	Timeout        Duration          `yaml:"timeout"`
	RemoteEmail    string            `yaml:"email_keyword"`
}

// Duration wraps time.Duration with YAML unmarshalling support.
type Duration struct {
	time.Duration
}

// UnmarshalYAML parses the duration from either a string (e.g. "30s") or an
// integer representing seconds.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		d.Duration = 0
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		var text string
		if err := node.Decode(&text); err == nil {
			if text == "" {
				d.Duration = 0
				return nil
			}
			dur, err := time.ParseDuration(text)
			if err == nil {
				d.Duration = dur
				return nil
			}
		}
		var seconds int64
		if err := node.Decode(&seconds); err == nil {
			d.Duration = time.Duration(seconds) * time.Second
			return nil
		}
		return fmt.Errorf("invalid duration %q", node.Value)
	default:
		return fmt.Errorf("unsupported YAML kind %d for duration", node.Kind)
	}
}

// LoadConfig reads and validates the synchronisation configuration.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyDefaults(&cfg)
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Local.ExportPath == "" {
		cfg.Local.ExportPath = "account-export.yaml"
	}
	if cfg.Remote.Port == 0 {
		cfg.Remote.Port = 22
	}
	if cfg.Remote.ExportPath == "" {
		cfg.Remote.ExportPath = "account-export.yaml"
	}
	if cfg.Remote.ImportPath == "" {
		cfg.Remote.ImportPath = cfg.Remote.ExportPath
	}
	if cfg.Remote.Timeout.Duration == 0 {
		cfg.Remote.Timeout.Duration = 30 * time.Second
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Local.DSN == "" {
		return fmt.Errorf("local.dsn must be configured")
	}
	if cfg.Remote.Address == "" {
		return fmt.Errorf("remote.address must be configured")
	}
	if cfg.Remote.User == "" {
		return fmt.Errorf("remote.user must be configured")
	}
	if cfg.Remote.AccountDir == "" {
		return fmt.Errorf("remote.account_dir must be configured")
	}
	if cfg.Remote.IdentityFile != "" {
		if _, err := os.Stat(cfg.Remote.IdentityFile); err != nil {
			return fmt.Errorf("remote.identity_file: %w", err)
		}
		abs, err := filepath.Abs(cfg.Remote.IdentityFile)
		if err == nil {
			cfg.Remote.IdentityFile = abs
		}
	}
	if cfg.Remote.KnownHostsFile != "" {
		if _, err := os.Stat(cfg.Remote.KnownHostsFile); err != nil {
			return fmt.Errorf("remote.known_hosts_file: %w", err)
		}
		abs, err := filepath.Abs(cfg.Remote.KnownHostsFile)
		if err == nil {
			cfg.Remote.KnownHostsFile = abs
		}
	}
	return nil
}
