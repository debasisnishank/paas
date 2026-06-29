package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage environment secrets (backed by HashiCorp Vault)",
}

var secretsSetCmd = &cobra.Command{
	Use:   "set <KEY=value>...",
	Short: "Set one or more secrets",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("setting %d secret(s) env=%s\n", len(args), env)
		// TODO: PUT /v1/.../envs/{env}/secrets
		// Secrets are stored in Vault; the API writes to Vault path, never our DB
		return errNotImplemented("secrets set")
	},
}

var secretsGetCmd = &cobra.Command{
	Use:   "get <KEY>",
	Short: "Print a secret value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("getting secret %q env=%s\n", args[0], env)
		// TODO: GET /v1/.../envs/{env}/secrets/{key}
		return errNotImplemented("secrets get")
	},
}

var secretsRmCmd = &cobra.Command{
	Use:   "rm <KEY>...",
	Short: "Remove one or more secrets",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("removing %d secret(s) env=%s\n", len(args), env)
		// TODO: DELETE /v1/.../envs/{env}/secrets/{key}
		return errNotImplemented("secrets rm")
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret keys (values masked)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("listing secrets env=%s\n", env)
		// TODO: GET /v1/.../envs/{env}/secrets
		return errNotImplemented("secrets list")
	},
}

func init() {
	secretsCmd.AddCommand(secretsSetCmd, secretsGetCmd, secretsRmCmd, secretsListCmd)
}
