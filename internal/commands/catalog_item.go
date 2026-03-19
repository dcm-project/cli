package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	catalogapi "github.com/dcm-project/catalog-manager/api/v1alpha1"

	"github.com/dcm-project/cli/internal/config"
	"github.com/dcm-project/cli/internal/output"
	"github.com/spf13/cobra"
)

var catalogItemTableDef = &output.TableDef{
	Headers: []string{"UID", "DISPLAY NAME", "SERVICE TYPE", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", ""}
		}
		var serviceType string
		if spec, ok := m["spec"].(map[string]any); ok {
			serviceType = stringifyValue(spec, "service_type")
		}
		return []string{
			stringifyValue(m, "uid"),
			stringifyValue(m, "display_name"),
			serviceType,
			stringifyValue(m, "create_time"),
		}
	},
}

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
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new catalog item",
		PreRunE: requiredFlagsPreRun,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fromFile, _ := cmd.Flags().GetString("from-file")

			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, catalogItemTableDef, "catalog item create")
			if err != nil {
				return err
			}

			body, err := parseInputFileAs[catalogapi.CreateCatalogItemJSONRequestBody](fromFile)
			if err != nil {
				return err
			}

			params := &catalogapi.CreateCatalogItemParams{}
			if id, _ := cmd.Flags().GetString("id"); id != "" {
				params.Id = &id
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.CreateCatalogItem(ctx, params, body)
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				return handleErrorResponse(resp, formatter)
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return formatter.FormatOne(result)
		},
	}

	cmd.Flags().String("from-file", "", "Path to catalog item YAML/JSON file (required)")
	_ = cmd.MarkFlagRequired("from-file")
	cmd.Flags().String("id", "", "Client-specified catalog item ID")

	return cmd
}

func newCatalogItemListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List catalog items",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			listCmd := "catalog item list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, catalogItemTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &catalogapi.ListCatalogItemsParams{}
			if serviceType, _ := cmd.Flags().GetString("service-type"); serviceType != "" {
				params.ServiceType = &serviceType
			}
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				params.MaxPageSize = &pageSize
			}
			if pageToken, _ := cmd.Flags().GetString("page-token"); pageToken != "" {
				params.PageToken = &pageToken
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.ListCatalogItems(ctx, params)
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return handleErrorResponse(resp, formatter)
			}

			var listResp struct {
				Results       []map[string]any `json:"results"`
				NextPageToken string           `json:"next_page_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			resources := make([]any, len(listResp.Results))
			for i, r := range listResp.Results {
				resources[i] = r
			}

			return formatter.FormatList(resources, listResp.NextPageToken)
		},
	}

	cmd.Flags().String("service-type", "", "Filter by service type")
	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")

	return cmd
}

func newCatalogItemGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get CATALOG_ITEM_ID",
		Short: "Get a catalog item by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, catalogItemTableDef, "catalog item get")
			if err != nil {
				return err
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetCatalogItem(ctx, args[0])
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return handleErrorResponse(resp, formatter)
			}

			var result map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return formatter.FormatOne(result)
		},
	}
}

func newCatalogItemDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete CATALOG_ITEM_ID",
		Short: "Delete a catalog item by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, catalogItemTableDef, "catalog item delete")
			if err != nil {
				return err
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.DeleteCatalogItem(ctx, args[0])
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				return handleErrorResponse(resp, formatter)
			}

			return formatter.FormatMessage(fmt.Sprintf("Catalog item %q deleted successfully.", args[0]))
		},
	}
}
