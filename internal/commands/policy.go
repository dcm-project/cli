package commands

import "github.com/spf13/cobra"

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
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new policy",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newPolicyListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List policies",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newPolicyGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get POLICY_ID",
		Short: "Get a policy by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newPolicyUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update POLICY_ID",
		Short: "Update an existing policy",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newPolicyDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete POLICY_ID",
		Short: "Delete a policy by ID",
		Args:  ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}
