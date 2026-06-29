package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/threemates/antariksh/cli/internal/manifest"
)

var (
	deployRegion  string
	deployService string
	deployWatch   bool
)

// deployRequest is the body POSTed to the deploy endpoint.
type deployRequest struct {
	Builder string `json:"builder"`
	Region  string `json:"region"`
	Env     string `json:"env"`
	Image   string `json:"image,omitempty"`
}

// deployment mirrors the API's deploy response (domain.Deployment + url).
type deployment struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	EnvID     string `json:"env_id"`
	Image     string `json:"image"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"`
}

var deployCmd = &cobra.Command{
	Use:   "deploy [path]",
	Short: "Deploy a service from the current directory",
	Long: `Reads platform.toml, builds the image (Nixpacks / Buildpack / Dockerfile),
pushes to the internal registry, and rolls out to the target environment.

A preview environment is created automatically for every git branch or PR.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		m, err := manifest.Load(filepath.Join(root, "platform.toml"))
		if err != nil {
			return fmt.Errorf("read manifest: %w", err)
		}
		svc, err := m.ServiceByName(deployService)
		if err != nil {
			return err
		}

		org := firstNonEmpty(orgSlug, m.App.Org)
		proj := firstNonEmpty(project, m.App.Project)
		region := firstNonEmpty(deployRegion, m.App.Region)

		req := deployRequest{Builder: m.Build.Builder, Region: region, Env: env}
		path := fmt.Sprintf("/v1/orgs/%s/projects/%s/services/%s/deploy", org, proj, svc.Name)

		var dep deployment
		if err := apiPost(cmd.Context(), path, req, &dep); err != nil {
			return err
		}

		fmt.Printf("deployment %s — %s/%s/%s (env=%s, region=%s) status=%s\n",
			dep.ID, org, proj, svc.Name, env, region, dep.Status)
		if dep.URL != "" {
			fmt.Printf("  live at %s\n", dep.URL)
		}
		if deployWatch && dep.URL == "" {
			fmt.Println("note: build + deploy log streaming not yet implemented (build pipeline pending)")
		}
		return nil
	},
}

func init() {
	deployCmd.Flags().StringVarP(&deployRegion, "region", "r", "", "target region (default: from platform.toml)")
	deployCmd.Flags().StringVarP(&deployService, "service", "s", "", "service name (default: first in platform.toml)")
	deployCmd.Flags().BoolVarP(&deployWatch, "watch", "w", true, "stream build and deploy logs")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
