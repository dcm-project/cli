package commands

import (
	"github.com/spf13/cobra"
)

func newCatalogItemCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "item",
		Short: "Manage catalog items",
	}

	cmd.AddCommand(newCatalogItemCreateCommand())
	cmd.AddCommand(newCatalogItemListCommand())
	cmd.AddCommand(newCatalogItemGetCommand())
	cmd.AddCommand(newCatalogItemDeleteCommand())

	return cmd
}

func newCatalogItemCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new catalog item",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newCatalogItemListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List catalog items",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newCatalogItemGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get CATALOG_ITEM_ID",
		Short: "Get a catalog item by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newCatalogItemDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete CATALOG_ITEM_ID",
		Short: "Delete a catalog item by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}
