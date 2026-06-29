package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialise a new project in the current directory",
	Long: `Creates a platform.toml manifest and authenticates the CLI.
If a git remote is detected, links the project to it for git-push-to-deploy.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			wd, _ := os.Getwd()
			name = wd // fallback; would prompt interactively
		}
		fmt.Printf("initialising project %q\n", name)
		// TODO: interactive prompt for org, region, runtime
		// TODO: POST /v1/orgs/{org}/projects
		// TODO: write platform.toml scaffold to cwd
		return errNotImplemented("init")
	},
}
