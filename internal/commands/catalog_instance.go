package commands

import (
	"github.com/spf13/cobra"
)

func newCatalogInstanceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Manage catalog item instances",
	}

	cmd.AddCommand(newCatalogInstanceCreateCommand())
	cmd.AddCommand(newCatalogInstanceListCommand())
	cmd.AddCommand(newCatalogInstanceGetCommand())
	cmd.AddCommand(newCatalogInstanceDeleteCommand())

	return cmd
}

func newCatalogInstanceCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new catalog item instance",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newCatalogInstanceListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List catalog item instances",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newCatalogInstanceGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get INSTANCE_ID",
		Short: "Get a catalog item instance by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newCatalogInstanceDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete INSTANCE_ID",
		Short: "Delete a catalog item instance by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}
