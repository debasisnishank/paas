package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/threemates/antariksh/cli/internal/manifest"
)

var (
	deployRegion  string
	deployService string
	deployWatch   bool
)

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

		path := fmt.Sprintf("/v1/orgs/%s/projects/%s/services/%s/deploy", org, proj, svc.Name)
		fields := map[string]string{
			"builder": m.Build.Builder,
			"region":  region,
			"env":     env,
		}

		// Stream a gzip-tar of the project dir as the build context. The server
		// builds it into a rootfs and boots a microVM.
		fmt.Printf("uploading source from %s …\n", root)
		pr, pw := io.Pipe()
		go func() { _ = pw.CloseWithError(tarGzDir(root, pw)) }()

		var dep deployment
		if err := apiPostMultipart(cmd.Context(), path, fields, "source", "source.tar.gz", pr, &dep); err != nil {
			return err
		}

		fmt.Printf("deployment %s — %s/%s/%s (env=%s, region=%s) status=%s\n",
			dep.ID, org, proj, svc.Name, env, region, dep.Status)
		if dep.URL != "" {
			fmt.Printf("  live at %s\n", dep.URL)
		}
		return nil
	},
}

// tarGzDir writes a gzip-compressed tar of root to w, skipping VCS and build
// junk. Paths in the archive are relative to root so it unpacks as a build
// context (Dockerfile at the top level).
func tarGzDir(root string, w io.Writer) error {
	gz := gzip.NewWriter(w)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if info.IsDir() {
			if path != root && skipDir[name] {
				return filepath.SkipDir
			}
			return nil // directories are implied by their file entries
		}
		if !info.Mode().IsRegular() {
			return nil // skip symlinks/devices
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = io.Copy(tw, f)
		return err
	})
}

// skipDir lists directory names left out of the upload context.
var skipDir = map[string]bool{
	".git":         true,
	"node_modules": true,
	"target":       true,
	"vendor":       true,
	".antctl":      true,
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
