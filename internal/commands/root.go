// Package commands defines the Cobra command tree for the dcm CLI.
package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/dcm-project/cli/internal/config"
	"github.com/spf13/cobra"
)

const (
	exitCodeRuntime = 1
	exitCodeUsage   = 2
)

// NewRootCommand creates the root `dcm` command with all subcommand groups
// and global flags registered.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "dcm",
		Short:         "DCM CLI - Data Center Management command-line tool",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(cmd)
			if err != nil {
				return err
			}
			cmd.SetContext(config.WithConfig(cmd.Context(), cfg))
			return nil
		},
	}

	// Wrap flag parsing errors as usage errors (exit code 2).
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &UsageError{Err: err}
	})

	// Global flags
	flags := cmd.PersistentFlags()
	flags.String("api-gateway-url", "http://localhost:9080", "API Gateway URL")
	flags.StringP("output", "o", "table", "Output format: table, json, yaml")
	flags.Int("timeout", 30, "Request timeout in seconds")
	flags.String("config", "", "Path to config file (default: ~/.dcm/config.yaml)")
	flags.String("tls-ca-cert", "", "Path to CA certificate file for TLS verification")
	flags.String("tls-client-cert", "", "Path to client certificate file for mTLS")
	flags.String("tls-client-key", "", "Path to client private key file for mTLS")
	flags.Bool("tls-skip-verify", false, "Skip TLS certificate verification")

	// Register subcommand groups
	cmd.AddCommand(newPolicyCommand())
	cmd.AddCommand(newCatalogCommand())
	cmd.AddCommand(newSPCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}

// Execute runs the root command and exits with the appropriate exit code.
func Execute() {
	cmd := NewRootCommand()
	err := cmd.Execute()
	if err != nil {
		var fmtErr *FormattedError
		if !errors.As(err, &fmtErr) {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err)
		}
		os.Exit(getExitCode(err))
	}
}

// getExitCode determines the exit code from an error.
func getExitCode(err error) int {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return exitCodeUsage
	}
	return exitCodeRuntime
}

// UsageError indicates a usage error (exit code 2).
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string {
	return e.Err.Error()
}

func (e *UsageError) Unwrap() error {
	return e.Err
}

// ExactArgs returns a cobra.PositionalArgs that wraps validation errors as UsageError.
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return &UsageError{Err: err}
		}
		return nil
	}
}

// requiredFlagsPreRun is a PreRunE hook that wraps Cobra's required-flag
// validation errors as UsageError (exit code 2). Cobra's own
// ValidateRequiredFlags call that follows PreRunE becomes a no-op.
func requiredFlagsPreRun(cmd *cobra.Command, _ []string) error {
	if err := cmd.ValidateRequiredFlags(); err != nil {
		return &UsageError{Err: err}
	}
	return nil
}
