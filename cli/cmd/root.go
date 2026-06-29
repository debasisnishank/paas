// Package cmd implements the antctl command-line interface.
//
// Command tree:
//   antctl init
//   antctl deploy [--env] [--region]
//   antctl logs   <service> [--follow]
//   antctl scale  <service> --count N | --zero
//   antctl secrets set|get|rm <key>
//   antctl db     branch|connect|restore|list
//   antctl ssh    <service>
//   antctl regions
//   antctl open   [service]
//
// DX target: flyctl / railway CLI grade.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	orgSlug string
	project string
	env     string
)

var rootCmd = &cobra.Command{
	Use:   "antctl",
	Short: "Antariksh Platform CLI",
	Long: `antctl — deploy and manage workloads on the Antariksh developer cloud.

Docs: https://docs.antariksh.in
`,
	SilenceUsage: true,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.antctl.yaml)")
	rootCmd.PersistentFlags().StringVarP(&orgSlug, "org", "o", "", "organisation slug")
	rootCmd.PersistentFlags().StringVarP(&project, "project", "p", "", "project slug")
	rootCmd.PersistentFlags().StringVarP(&env, "env", "e", "production", "environment name")

	rootCmd.AddCommand(
		loginCmd,
		initCmd,
		deployCmd,
		logsCmd,
		scaleCmd,
		secretsCmd,
		dbCmd,
		sshCmd,
		regionsCmd,
		openCmd,
	)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, _ := os.UserHomeDir()
		viper.AddConfigPath(home)
		viper.SetConfigName(".antctl")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("ANTARIKSH")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

func apiBase() string {
	if u := viper.GetString("api_url"); u != "" {
		return u
	}
	return "https://api.antariksh.in"
}
