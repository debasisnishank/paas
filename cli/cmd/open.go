package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open [service]",
	Short: "Open the deployed service URL in a browser",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := ""
		if len(args) > 0 {
			svc = args[0]
		}
		// TODO: GET /v1/.../services/{svc} → url field
		url := fmt.Sprintf("https://%s.antariksh.app", svc) // placeholder
		fmt.Printf("opening %s\n", url)
		return openBrowser(url)
	},
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "linux":
		cmd, args = "xdg-open", []string{url}
	default:
		return fmt.Errorf("unsupported OS; open %s manually", url)
	}
	return exec.Command(cmd, args...).Start()
}

// errNotImplemented is a shared sentinel used by all stub commands.
func errNotImplemented(name string) error {
	return fmt.Errorf("%s: not yet implemented — contributions welcome", name)
}
