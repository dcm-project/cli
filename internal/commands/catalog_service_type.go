package commands

import (
	"github.com/spf13/cobra"
)

func newCatalogServiceTypeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-type",
		Short: "Manage service types",
	}

	cmd.AddCommand(newServiceTypeListCommand())
	cmd.AddCommand(newServiceTypeGetCommand())

	return cmd
}

func newServiceTypeListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List service types",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newServiceTypeGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get SERVICE_TYPE_ID",
		Short: "Get a service type by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}
