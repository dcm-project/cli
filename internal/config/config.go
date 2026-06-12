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
)

const defaultControlPlaneURL = "http://localhost:8080"

type contextKey struct{}

// WithConfig stores a Config in the given context.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext retrieves the Config from a context. Returns nil if not present.
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
}

// Load reads configuration from file, environment variables, and command-line
// flags in the precedence order: flags > env vars > config file > defaults.
func Load(cmd *cobra.Command) (*Config, error) {
	v := viper.New()

	v.SetDefault("output-format", "table")
	v.SetDefault("timeout", 30)
	v.SetDefault("tls-ca-cert", "")
	v.SetDefault("tls-client-cert", "")
	v.SetDefault("tls-client-key", "")
	v.SetDefault("tls-skip-verify", false)

	v.SetEnvPrefix("DCM")
	v.MustBindEnv("output-format", "DCM_OUTPUT_FORMAT")
	v.MustBindEnv("timeout", "DCM_TIMEOUT")
	v.MustBindEnv("tls-ca-cert", "DCM_TLS_CA_CERT")
	v.MustBindEnv("tls-client-cert", "DCM_TLS_CLIENT_CERT")
	v.MustBindEnv("tls-client-key", "DCM_TLS_CLIENT_KEY")
	v.MustBindEnv("tls-skip-verify", "DCM_TLS_SKIP_VERIFY")

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

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	if cmd != nil {
		if err := bindFlags(v, cmd); err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	cfg.ControlPlaneURL = resolveControlPlaneURL(v, cmd)
	return &cfg, nil
}

func resolveControlPlaneURL(v *viper.Viper, cmd *cobra.Command) string {
	if cmd != nil {
		if f := cmd.Root().PersistentFlags().Lookup("control-plane-url"); f != nil && f.Changed {
			return f.Value.String()
		}
		if f := cmd.Root().PersistentFlags().Lookup("api-gateway-url"); f != nil && f.Changed {
			return f.Value.String()
		}
	}

	if u := os.Getenv("DCM_CONTROL_PLANE_URL"); u != "" {
		return u
	}
	if u := os.Getenv("DCM_API_GATEWAY_URL"); u != "" {
		return u
	}

	if v.InConfig("control-plane-url") {
		return v.GetString("control-plane-url")
	}
	if v.InConfig("api-gateway-url") {
		return v.GetString("api-gateway-url")
	}

	return defaultControlPlaneURL
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

// bindFlags binds only flags that were explicitly set by the user, so that
// unset flags don't override environment variables or config file values.
func bindFlags(v *viper.Viper, cmd *cobra.Command) error {
	flagToKey := map[string]string{
		"output":          "output-format",
		"timeout":         "timeout",
		"tls-ca-cert":     "tls-ca-cert",
		"tls-client-cert": "tls-client-cert",
		"tls-client-key":  "tls-client-key",
		"tls-skip-verify": "tls-skip-verify",
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
