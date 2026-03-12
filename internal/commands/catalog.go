package commands

import (
	"github.com/spf13/cobra"
)

func newCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage catalog resources",
	}

	cmd.AddCommand(newCatalogServiceTypeCommand())
	cmd.AddCommand(newCatalogItemCommand())
	cmd.AddCommand(newCatalogInstanceCommand())

	return cmd
}
