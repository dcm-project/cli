package commands

import (
	"fmt"

	"github.com/dcm-project/cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version and build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := version.Get()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dcm version %s\n", info.Version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", info.Commit)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", info.BuildTime)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  go:     %s\n", info.GoVersion)
			return nil
		},
	}
}
