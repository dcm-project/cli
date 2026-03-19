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

var serviceTypeTableDef = &output.TableDef{
	Headers: []string{"UID", "SERVICE TYPE", "API VERSION", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", ""}
		}
		return []string{
			stringifyValue(m, "uid"),
			stringifyValue(m, "service_type"),
			stringifyValue(m, "api_version"),
			stringifyValue(m, "create_time"),
		}
	},
}

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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List service types",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			listCmd := "catalog service-type list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, serviceTypeTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &catalogapi.ListServiceTypesParams{}
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

			resp, err := client.ListServiceTypes(ctx, params)
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

	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")

	return cmd
}

func newServiceTypeGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get SERVICE_TYPE_ID",
		Short: "Get a service type by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, serviceTypeTableDef, "catalog service-type get")
			if err != nil {
				return err
			}

			client, err := newCatalogClient(cfg)
			if err != nil {
				return fmt.Errorf("creating catalog client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetServiceType(ctx, args[0])
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
