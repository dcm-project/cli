// Package config manages CLI configuration with file persistence,
// environment variable overrides, and command-line flag overrides.
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

const defaultControlPlaneURL = "http://localhost:8080"

type contextKey struct{}

// WithConfig stores a Config in the given context.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext retrieves a Config from a context. Returns nil if not present.
func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(contextKey{}).(*Config)
	return cfg
}

// FromCommand retrieves the Config from a cobra.Command's context.
func FromCommand(cmd *cobra.Command) *Config {
	return FromContext(cmd.Context())
}

// Config holds the resolved CLI configuration.
type Config struct {
	ControlPlaneURL string `yaml:"control-plane-url" mapstructure:"control-plane-url"`
	OutputFormat    string `yaml:"output-format" mapstructure:"output-format"`
	Timeout         int    `yaml:"timeout" mapstructure:"timeout"`
	TLSCACert       string `yaml:"tls-ca-cert" mapstructure:"tls-ca-cert"`
	TLSClientCert   string `yaml:"tls-client-cert" mapstructure:"tls-client-cert"`
	TLSClientKey    string `yaml:"tls-client-key" mapstructure:"tls-client-key"`
	TLSSkipVerify   bool   `yaml:"tls-skip-verify" mapstructure:"tls-skip-verify"`
	IssuerURL       string `yaml:"issuer-url" mapstructure:"issuer-url"`
	Token           string `yaml:"-" mapstructure:"token"`
}

// Load reads configuration from file, environment variables, and command-line
// flags in the precedence order: flags > env vars > config file > defaults.
func Load(cmd *cobra.Command) (*Config, error) {
	v := viper.New()

	// Built-in defaults (REQ-CFG-050)
	v.SetDefault("control-plane-url", defaultControlPlaneURL)
	v.SetDefault("output-format", "table")
	v.SetDefault("timeout", 30)
	v.SetDefault("tls-ca-cert", "")
	v.SetDefault("tls-client-cert", "")
	v.SetDefault("tls-client-key", "")
	v.SetDefault("tls-skip-verify", false)
	v.SetDefault("issuer-url", "")
	v.SetDefault("token", "")

	// Environment variable binding (REQ-CFG-030)
	v.SetEnvPrefix("DCM")
	v.MustBindEnv("control-plane-url", "DCM_CONTROL_PLANE_URL")
	v.MustBindEnv("output-format", "DCM_OUTPUT_FORMAT")
	v.MustBindEnv("timeout", "DCM_TIMEOUT")
	v.MustBindEnv("tls-ca-cert", "DCM_TLS_CA_CERT")
	v.MustBindEnv("tls-client-cert", "DCM_TLS_CLIENT_CERT")
	v.MustBindEnv("tls-client-key", "DCM_TLS_CLIENT_KEY")
	v.MustBindEnv("tls-skip-verify", "DCM_TLS_SKIP_VERIFY")
	v.MustBindEnv("issuer-url", "DCM_ISSUER_URL")
	v.MustBindEnv("token", "DCM_TOKEN")

	// Config file path (REQ-CFG-010, REQ-CFG-020)
	configPath := configFilePath(cmd)
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("unable to determine home directory: %w", err)
		}
		v.SetConfigFile(filepath.Join(home, ".dcm", "config.yaml"))
	}

	// Read config file — ignore "not found" errors (REQ-CFG-070)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	// Bind CLI flags so they override env vars and config file (REQ-CFG-040)
	if cmd != nil {
		if err := bindFlags(v, cmd); err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}

// configFilePath resolves the config file path from the --config flag
// or the DCM_CONFIG environment variable.
func configFilePath(cmd *cobra.Command) string {
	if cmd != nil {
		f := cmd.Root().PersistentFlags().Lookup("config")
		if f != nil && f.Changed {
			return f.Value.String()
		}
	}
	if v := os.Getenv("DCM_CONFIG"); v != "" {
		return v
	}
	return ""
}

// ConfigPath returns the config file path that Load would use for cmd:
// --config / DCM_CONFIG if set, otherwise ~/.dcm/config.yaml.
func ConfigPath(cmd *cobra.Command) string {
	if path := configFilePath(cmd); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".dcm", "config.yaml")
}

// SaveConfig merges the provided key-value pairs into the config file at path,
// creating the file and parent directory if they don't exist. Existing values
// not present in the values map are preserved. If path is empty, writes to
// ~/.dcm/config.yaml.
func SaveConfig(path string, values map[string]string) error {
	configPath := path
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("unable to determine home directory: %w", err)
		}
		configPath = filepath.Join(home, ".dcm", "config.yaml")
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	existing := make(map[string]any)
	data, err := os.ReadFile(configPath)
	if err == nil {
		if yamlErr := yaml.Unmarshal(data, &existing); yamlErr != nil {
			return fmt.Errorf("parsing existing config: %w", yamlErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading config file: %w", err)
	}
	if existing == nil {
		existing = make(map[string]any)
	}

	for k, v := range values {
		existing[k] = v
	}

	out, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("saving config file: %w", err)
	}

	return nil
}

// bindFlags binds only flags that were explicitly set by the user, so that
// unset flags don't override environment variables or config file values.
func bindFlags(v *viper.Viper, cmd *cobra.Command) error {
	flagToKey := map[string]string{
		"control-plane-url": "control-plane-url",
		"output":            "output-format",
		"timeout":           "timeout",
		"tls-ca-cert":       "tls-ca-cert",
		"tls-client-cert":   "tls-client-cert",
		"tls-client-key":    "tls-client-key",
		"tls-skip-verify":   "tls-skip-verify",
		"issuer-url":        "issuer-url",
		"token":             "token",
	}

	for flagName, configKey := range flagToKey {
		f := cmd.Root().PersistentFlags().Lookup(flagName)
		if f != nil && f.Changed {
			if err := v.BindPFlag(configKey, f); err != nil {
				return fmt.Errorf("binding flag %s: %w", flagName, err)
			}
		}
	}
	return nil
}
