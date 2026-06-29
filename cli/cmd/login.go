package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	loginEmail  string
	loginAPIKey string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Antariksh API and store a bearer token",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if loginEmail == "" || loginAPIKey == "" {
			return fmt.Errorf("--email and --api-key are required")
		}
		token, email, err := doLogin(cmd.Context(), loginEmail, loginAPIKey)
		if err != nil {
			return err
		}
		if err := saveToken(token, email); err != nil {
			return fmt.Errorf("saving token: %w", err)
		}
		fmt.Printf("logged in as %s — token saved to %s\n", email, configPath())
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "account email")
	loginCmd.Flags().StringVar(&loginAPIKey, "api-key", "", "API key")
}

func doLogin(ctx context.Context, email, apiKey string) (token, gotEmail string, err error) {
	var resp struct {
		Token     string `json:"token"`
		Email     string `json:"email"`
		ExpiresAt int64  `json:"expires_at"`
	}
	body := map[string]string{"email": email, "api_key": apiKey}
	if err := apiPost(ctx, "/v1/auth/login", body, &resp); err != nil {
		return "", "", err
	}
	return resp.Token, resp.Email, nil
}

// configPath is the antctl config file we read/write (default ~/.antctl.yaml).
func configPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".antctl.yaml")
}

func saveToken(token, email string) error {
	viper.Set("token", token)
	viper.Set("email", email)
	return viper.WriteConfigAs(configPath())
}
