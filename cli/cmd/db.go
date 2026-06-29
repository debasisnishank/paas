package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage serverless Postgres databases and branches",
	Long: `Serverless Postgres with O(1) copy-on-write branching (Neon engine).
Each PR preview environment gets its own branch — cheap and instant.`,
}

var dbBranchCmd = &cobra.Command{
	Use:   "branch <db-name> [branch-name]",
	Short: "Create a CoW branch of a database",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]
		branch := ""
		if len(args) > 1 {
			branch = args[1]
		}
		fmt.Printf("branching db=%s new-branch=%q\n", dbName, branch)
		// TODO: POST /v1/.../databases/{db}/branches
		// Neon engine: Pageserver timeline fork — O(1), nearly free
		return errNotImplemented("db branch")
	},
}

var dbConnectCmd = &cobra.Command{
	Use:   "connect <db-name>",
	Short: "Open a psql session to a database",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("connecting to db=%s env=%s\n", args[0], env)
		// TODO: GET /v1/.../databases/{db}/connection-string → exec psql
		return errNotImplemented("db connect")
	},
}

var dbRestoreCmd = &cobra.Command{
	Use:   "restore <db-name> --to <timestamp>",
	Short: "Restore database to a point in time (PITR)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		to, _ := cmd.Flags().GetString("to")
		fmt.Printf("restoring db=%s to=%s\n", args[0], to)
		// TODO: POST /v1/.../databases/{db}/restore {"target_time": to}
		// Triggers Temporal RestorePITR workflow: WAL replay from MinIO archive
		return errNotImplemented("db restore")
	},
}

var dbListCmd = &cobra.Command{
	Use:   "list",
	Short: "List databases in the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("listing databases project=%s env=%s\n", project, env)
		// TODO: GET /v1/.../databases
		return errNotImplemented("db list")
	},
}

func init() {
	dbRestoreCmd.Flags().String("to", "", "target timestamp (RFC3339), e.g. 2025-01-15T10:30:00Z")
	_ = dbRestoreCmd.MarkFlagRequired("to")
	dbCmd.AddCommand(dbBranchCmd, dbConnectCmd, dbRestoreCmd, dbListCmd)
}
