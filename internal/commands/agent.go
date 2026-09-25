package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	agentapi "github.com/dcm-project/control-plane/api/agent/v1alpha1"

	"github.com/dcm-project/cli/internal/config"
	"github.com/dcm-project/cli/internal/output"
	"github.com/spf13/cobra"
)

var agentTableDef = &output.TableDef{
	Headers: []string{"ID", "NAME", "ENVIRONMENT", "HEALTH", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", "", ""}
		}
		return []string{
			stringifyValue(m, "agent_id"),
			stringifyValue(m, "name"),
			stringifyValue(m, "environment"),
			stringifyValue(m, "health_status"),
			stringifyValue(m, "create_time"),
		}
	},
}

func newAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage environment agents",
	}

	cmd.AddCommand(newAgentListCommand())
	cmd.AddCommand(newAgentGetCommand())

	return cmd
}

func newAgentListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environment agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			listCmd := "agent list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, agentTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &agentapi.ListAgentsParams{}
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				maxPageSize := int(pageSize)
				params.MaxPageSize = &maxPageSize
			}
			if pageToken, _ := cmd.Flags().GetString("page-token"); pageToken != "" {
				params.PageToken = &pageToken
			}
			if healthStatus, _ := cmd.Flags().GetString("health-status"); healthStatus != "" {
				status := agentapi.ListAgentsParamsHealthStatus(healthStatus)
				params.HealthStatus = &status
			}

			client, err := newAgentClient(cfg)
			if err != nil {
				return fmt.Errorf("creating agent client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.ListAgents(ctx, params)
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return handleErrorResponse(resp, formatter)
			}

			var listResp struct {
				Agents        []map[string]any `json:"agents"`
				NextPageToken string           `json:"next_page_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			resources := make([]any, len(listResp.Agents))
			for i, r := range listResp.Agents {
				resources[i] = r
			}

			return formatter.FormatList(resources, listResp.NextPageToken)
		},
	}

	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")
	cmd.Flags().String("health-status", "", "Filter by health status (ready, congested, unavailable)")

	return cmd
}

func newAgentGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get AGENT_ID",
		Short: "Get an environment agent by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, agentTableDef, "agent get")
			if err != nil {
				return err
			}

			client, err := newAgentClient(cfg)
			if err != nil {
				return fmt.Errorf("creating agent client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetAgent(ctx, args[0])
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
