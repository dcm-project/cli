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

var catalogInstanceTableDef = &output.TableDef{
	Headers: []string{"UID", "DISPLAY NAME", "CATALOG ITEM", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", ""}
		}
		var catalogItemID string
		if spec, ok := m["spec"].(map[string]any); ok {
			catalogItemID = stringifyValue(spec, "catalog_item_id")
		}
		return []string{
			stringifyValue(m, "uid"),
			stringifyValue(m, "display_name"),
			catalogItemID,
			stringifyValue(m, "create_time"),
		}
	},
}

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
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new catalog item instance",
		PreRunE: requiredFlagsPreRun,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fromFile, _ := cmd.Flags().GetString("from-file")

			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, catalogInstanceTableDef, "catalog instance create")
			if err != nil {
				return err
			}

			body, err := parseInputFileAs[catalogapi.CreateCatalogItemInstanceJSONRequestBody](fromFile)
			if err != nil {
				return err
			}

			params := &catalogapi.CreateCatalogItemInstanceParams{}
			if id, _ := cmd.Flags().GetString("id"); id != "" {
				params.Id = &id
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.CreateCatalogItemInstance(ctx, params, body)
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

	cmd.Flags().String("from-file", "", "Path to instance YAML/JSON file (required)")
	_ = cmd.MarkFlagRequired("from-file")
	cmd.Flags().String("id", "", "Client-specified instance ID")

	return cmd
}

func newCatalogInstanceListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List catalog item instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			listCmd := "catalog instance list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, catalogInstanceTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &catalogapi.ListCatalogItemInstancesParams{}
			if catalogItemID, _ := cmd.Flags().GetString("catalog-item-id"); catalogItemID != "" {
				params.CatalogItemId = &catalogItemID
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

			resp, err := client.ListCatalogItemInstances(ctx, params)
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

	cmd.Flags().String("catalog-item-id", "", "Filter by catalog item ID")
	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")

	return cmd
}

func newCatalogInstanceGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get INSTANCE_ID",
		Short: "Get a catalog item instance by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, catalogInstanceTableDef, "catalog instance get")
			if err != nil {
				return err
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetCatalogItemInstance(ctx, args[0])
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

func newCatalogInstanceDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete INSTANCE_ID",
		Short: "Delete a catalog item instance by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, catalogInstanceTableDef, "catalog instance delete")
			if err != nil {
				return err
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.DeleteCatalogItemInstance(ctx, args[0])
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				return handleErrorResponse(resp, formatter)
			}

			return formatter.FormatMessage(fmt.Sprintf("Catalog item instance %q deleted successfully.", args[0]))
		},
	}
}
