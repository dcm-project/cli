package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dcm-project/cli/internal/auth"
	"github.com/dcm-project/cli/internal/config"
	"github.com/spf13/cobra"
)

func newLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the DCM control plane",
		Long:  "Authenticate with the DCM control plane using OIDC device authorization flow.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)
			if cfg.IssuerURL == "" {
				return &UsageError{Err: fmt.Errorf("--issuer-url is required (or set DCM_ISSUER_URL)")}
			}

			// Plain client: OIDC discovery/device/token must not go through
			// AuthTransport (expired stored tokens would deadlock on refresh).
			httpClient, err := buildPlainHTTPClient(cfg)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			tokenData, err := auth.DeviceLogin(ctx, cfg.IssuerURL, httpClient, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			store, err := auth.NewTokenStore()
			if err != nil {
				return fmt.Errorf("initializing credential store: %w", err)
			}
			if err := store.Save(cfg.IssuerURL, tokenData); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}

			configValues := map[string]string{
				"issuer-url": cfg.IssuerURL,
			}
			if controlPlaneURLExplicitlySet(cmd) {
				configValues["control-plane-url"] = cfg.ControlPlaneURL
			}
			if err := config.SaveConfig(config.ConfigPath(cmd), configValues); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not save config: %v\n", err)
			}

			username := auth.PreferredUsername(tokenData.AccessToken)
			ttl := time.Until(tokenData.Expiry).Round(time.Second)
			if username != "" {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Logged in as %s (token expires in %s; auto-refresh enabled)\n", username, ttl)
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Logged in successfully (token expires in %s; auto-refresh enabled)\n", ttl)
			}

			return nil
		},
	}
}

// controlPlaneURLExplicitlySet reports whether the user provided a control-plane
// URL via --control-plane-url or DCM_CONTROL_PLANE_URL. The built-in default
// alone does not count as "set" for login config persistence (REQ-LGN-120).
func controlPlaneURLExplicitlySet(cmd *cobra.Command) bool {
	if f := cmd.Root().PersistentFlags().Lookup("control-plane-url"); f != nil && f.Changed {
		return true
	}
	return os.Getenv("DCM_CONTROL_PLANE_URL") != ""
}
