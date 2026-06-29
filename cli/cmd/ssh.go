package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <service>",
	Short: "Open an interactive shell into a running microVM",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := args[0]
		fmt.Printf("ssh into %s env=%s\n", svc, env)
		// TODO: GET /v1/.../services/{svc}/ssh-token → short-lived WireGuard peer creds
		// TODO: exec ssh via the platform 6PN address
		return errNotImplemented("ssh")
	},
}
