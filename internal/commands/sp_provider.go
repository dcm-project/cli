package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	spmapi "github.com/dcm-project/service-provider-manager/api/v1alpha1/provider"

	"github.com/dcm-project/cli/internal/config"
	"github.com/dcm-project/cli/internal/output"
	"github.com/spf13/cobra"
)

var spProviderTableDef = &output.TableDef{
	Headers: []string{"ID", "NAME", "SERVICE TYPE", "STATUS", "HEALTH", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", "", "", ""}
		}
		return []string{
			stringifyValue(m, "id"),
			stringifyValue(m, "name"),
			stringifyValue(m, "service_type"),
			stringifyValue(m, "status"),
			stringifyValue(m, "health_status"),
			stringifyValue(m, "create_time"),
		}
	},
}

func newSPProviderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage SP providers",
	}

	cmd.AddCommand(newSPProviderListCommand())
	cmd.AddCommand(newSPProviderGetCommand())

	return cmd
}

func newSPProviderListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SP providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			listCmd := "sp provider list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, spProviderTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &spmapi.ListProvidersParams{}
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				maxPageSize := int(pageSize)
				params.MaxPageSize = &maxPageSize
			}
			if pageToken, _ := cmd.Flags().GetString("page-token"); pageToken != "" {
				params.PageToken = &pageToken
			}
			if providerType, _ := cmd.Flags().GetString("type"); providerType != "" {
				params.Type = &providerType
			}

			client, err := newSPProviderClient(cfg)
			if err != nil {
				return fmt.Errorf("creating SP provider client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.ListProviders(ctx, params)
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return handleErrorResponse(resp, formatter)
			}

			var listResp struct {
				Providers     []map[string]any `json:"providers"`
				NextPageToken string           `json:"next_page_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			resources := make([]any, len(listResp.Providers))
			for i, r := range listResp.Providers {
				resources[i] = r
			}

			return formatter.FormatList(resources, listResp.NextPageToken)
		},
	}

	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")
	cmd.Flags().String("type", "", "Filter by service type")

	return cmd
}

func newSPProviderGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get PROVIDER_ID",
		Short: "Get an SP provider by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, spProviderTableDef, "sp provider get")
			if err != nil {
				return err
			}

			client, err := newSPProviderClient(cfg)
			if err != nil {
				return fmt.Errorf("creating SP provider client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetProvider(ctx, args[0])
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
