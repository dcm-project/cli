package commands

import (
	"encoding/json"
	"fmt"
	"net/http"

	policyapi "github.com/dcm-project/policy-manager/api/v1alpha1"
	policyclient "github.com/dcm-project/policy-manager/pkg/client"

	"github.com/dcm-project/cli/internal/config"
	"github.com/dcm-project/cli/internal/output"
	"github.com/spf13/cobra"
)

var policyTableDef = &output.TableDef{
	Headers: []string{"ID", "DISPLAY NAME", "TYPE", "PRIORITY", "ENABLED", "CREATED"},
	RowFunc: func(resource any) []string {
		m, ok := resource.(map[string]any)
		if !ok {
			return []string{"", "", "", "", "", ""}
		}
		return []string{
			stringifyValue(m, "id"),
			stringifyValue(m, "display_name"),
			stringifyValue(m, "policy_type"),
			stringifyValue(m, "priority"),
			stringifyValue(m, "enabled"),
			stringifyValue(m, "create_time"),
		}
	},
}

func newPolicyClient(cfg *config.Config) (*policyclient.Client, error) {
	httpClient, err := buildHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return policyclient.NewClient(apiBaseURL(cfg), policyclient.WithHTTPClient(httpClient))
}

func newPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage policies",
	}

	cmd.AddCommand(newPolicyCreateCommand())
	cmd.AddCommand(newPolicyListCommand())
	cmd.AddCommand(newPolicyGetCommand())
	cmd.AddCommand(newPolicyUpdateCommand())
	cmd.AddCommand(newPolicyDeleteCommand())

	return cmd
}

func newPolicyCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new policy",
		PreRunE: requiredFlagsPreRun,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fromFile, _ := cmd.Flags().GetString("from-file")

			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, policyTableDef, "policy create")
			if err != nil {
				return err
			}

			body, err := parseInputFileAs[policyapi.CreatePolicyJSONRequestBody](fromFile)
			if err != nil {
				return err
			}

			params := &policyapi.CreatePolicyParams{}
			if id, _ := cmd.Flags().GetString("id"); id != "" {
				params.Id = &id
			}

			client, err := newPolicyClient(cfg)
			if err != nil {
				return fmt.Errorf("creating policy client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.CreatePolicy(ctx, params, body)
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

	cmd.Flags().String("from-file", "", "Path to policy YAML/JSON file (required)")
	_ = cmd.MarkFlagRequired("from-file")
	cmd.Flags().String("id", "", "Client-specified policy ID")

	return cmd
}

func newPolicyListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List policies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromCommand(cmd)

			// Build the pagination hint command string.
			listCmd := "policy list"
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				listCmd += fmt.Sprintf(" --page-size %d", pageSize)
			}

			formatter, err := newFormatter(cmd, policyTableDef, listCmd)
			if err != nil {
				return err
			}

			params := &policyapi.ListPoliciesParams{}
			if filter, _ := cmd.Flags().GetString("filter"); filter != "" {
				params.Filter = &filter
			}
			if orderBy, _ := cmd.Flags().GetString("order-by"); orderBy != "" {
				params.OrderBy = &orderBy
			}
			if pageSize, _ := cmd.Flags().GetInt32("page-size"); pageSize > 0 {
				params.MaxPageSize = &pageSize
			}
			if pageToken, _ := cmd.Flags().GetString("page-token"); pageToken != "" {
				params.PageToken = &pageToken
			}

			client, err := newPolicyClient(cfg)
			if err != nil {
				return fmt.Errorf("creating policy client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.ListPolicies(ctx, params)
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return handleErrorResponse(resp, formatter)
			}

			var listResp struct {
				Policies      []map[string]any `json:"policies"`
				NextPageToken string           `json:"next_page_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			resources := make([]any, len(listResp.Policies))
			for i, r := range listResp.Policies {
				resources[i] = r
			}

			return formatter.FormatList(resources, listResp.NextPageToken)
		},
	}

	cmd.Flags().String("filter", "", "CEL filter expression")
	cmd.Flags().String("order-by", "", "Order field and direction")
	cmd.Flags().Int32("page-size", 0, "Maximum results per page")
	cmd.Flags().String("page-token", "", "Token for next page")

	return cmd
}

func newPolicyGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get POLICY_ID",
		Short: "Get a policy by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, policyTableDef, "policy get")
			if err != nil {
				return err
			}

			client, err := newPolicyClient(cfg)
			if err != nil {
				return fmt.Errorf("creating policy client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.GetPolicy(ctx, args[0])
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

func newPolicyUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update POLICY_ID",
		Short:   "Update an existing policy",
		Args:    ExactArgs(1),
		PreRunE: requiredFlagsPreRun,
		RunE: func(cmd *cobra.Command, args []string) error {
			fromFile, _ := cmd.Flags().GetString("from-file")

			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, policyTableDef, "policy update")
			if err != nil {
				return err
			}

			body, err := parseInputFileAs[policyapi.UpdatePolicyApplicationMergePatchPlusJSONRequestBody](fromFile)
			if err != nil {
				return err
			}

			client, err := newPolicyClient(cfg)
			if err != nil {
				return fmt.Errorf("creating policy client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.UpdatePolicyWithApplicationMergePatchPlusJSONBody(ctx, args[0], body)
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

	cmd.Flags().String("from-file", "", "Path to patch YAML/JSON file (required)")
	_ = cmd.MarkFlagRequired("from-file")

	return cmd
}

func newPolicyDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete POLICY_ID",
		Short: "Delete a policy by ID",
		Args:  ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromCommand(cmd)
			formatter, err := newFormatter(cmd, policyTableDef, "policy delete")
			if err != nil {
				return err
			}

			client, err := newPolicyClient(cfg)
			if err != nil {
				return fmt.Errorf("creating policy client: %w", err)
			}

			ctx, cancel := requestContext(cmd)
			defer cancel()

			resp, err := client.DeletePolicy(ctx, args[0])
			if err != nil {
				return connectionError(err, cfg)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				return handleErrorResponse(resp, formatter)
			}

			return formatter.FormatMessage(fmt.Sprintf("Policy %q deleted successfully.", args[0]))
		},
	}
}
