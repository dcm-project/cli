package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	sprmapi "github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"

	"github.com/dcm-project/cli/internal/config"
	"github.com/dcm-project/cli/internal/output"
	"github.com/spf13/cobra"
)

var spResourceTableDef = &output.TableDef{
	Headers: []string{"ID", "PROVIDER", "STATUS", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", ""}
		}
		return []string{
			stringifyValue(m, "id"),
			stringifyValue(m, "provider_name"),
			stringifyValue(m, "status"),
			stringifyValue(m, "create_time"),
		}
	},
}

func newSPResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Manage SP resources",
	}

	cmd.AddCommand(newSPResourceListCommand())
	cmd.AddCommand(newSPResourceGetCommand())

	return cmd
}

func newSPResourceListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SP resources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			listCmd := "sp resource list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, spResourceTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &sprmapi.ListInstancesParams{}
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				maxPageSize := int(pageSize)
				params.MaxPageSize = &maxPageSize
			}
			if pageToken, _ := cmd.Flags().GetString("page-token"); pageToken != "" {
				params.PageToken = &pageToken
			}
			if provider, _ := cmd.Flags().GetString("provider"); provider != "" {
				params.Provider = &provider
			}

			client, err := newSPResourceClient(cfg)
			if err != nil {
				return fmt.Errorf("creating SP resource client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.ListInstances(ctx, params)
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return handleErrorResponse(resp, formatter)
			}

			var listResp struct {
				Instances     []map[string]any `json:"instances"`
				NextPageToken string           `json:"next_page_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			resources := make([]any, len(listResp.Instances))
			for i, r := range listResp.Instances {
				resources[i] = r
			}

			return formatter.FormatList(resources, listResp.NextPageToken)
		},
	}

	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")
	cmd.Flags().String("provider", "", "Filter by provider")

	return cmd
}

func newSPResourceGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get INSTANCE_ID",
		Short: "Get an SP resource by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, spResourceTableDef, "sp resource get")
			if err != nil {
				return err
			}

			client, err := newSPResourceClient(cfg)
			if err != nil {
				return fmt.Errorf("creating SP resource client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetInstance(ctx, args[0])
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
