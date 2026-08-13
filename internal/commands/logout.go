package commands

import (
	"fmt"

	"github.com/dcm-project/cli/internal/auth"
	"github.com/dcm-project/cli/internal/config"
	"github.com/spf13/cobra"
)

func newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored authentication credentials",
		Long:  "Revoke stored tokens and clear authentication credentials.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)
			if cfg.IssuerURL == "" {
				return &UsageError{Err: fmt.Errorf("--issuer-url is required (or set DCM_ISSUER_URL)")}
			}

			store, err := auth.NewTokenStore()
			if err != nil {
				return fmt.Errorf("initializing credential store: %w", err)
			}
			tokenData, err := store.Load(cfg.IssuerURL)
			if err != nil {
				return fmt.Errorf("reading stored credentials: %w", err)
			}

			if tokenData == nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No stored credentials found")
				return nil
			}

			if tokenData.RefreshToken != "" {
				httpClient, err := buildPlainHTTPClient(cfg)
				if err != nil {
					return err
				}
				ctx, cancel := requestContext(cmd)
				defer cancel()

				if err := auth.RevokeToken(ctx, cfg.IssuerURL, tokenData.RefreshToken, httpClient); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: token revocation failed: %v\n", err)
				}
			}

			if err := store.Delete(cfg.IssuerURL); err != nil {
				return fmt.Errorf("clearing stored credentials: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Logged out successfully")
			return nil
		},
	}
}
