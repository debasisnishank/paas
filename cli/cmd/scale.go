package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	scaleCount  int
	scaleZero   bool
	scaleRegion string
)

var scaleCmd = &cobra.Command{
	Use:   "scale <service>",
	Short: "Scale a service to N replicas or suspend it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := args[0]
		if scaleZero {
			fmt.Printf("suspending %s (scale-to-zero snapshot)\n", svc)
			// TODO: PATCH .../services/{svc}/scale {"replicas": 0}
		} else {
			fmt.Printf("scaling %s → count=%d region=%s\n", svc, scaleCount, scaleRegion)
			// TODO: PATCH .../services/{svc}/scale {"replicas": N, "region": R}
		}
		return errNotImplemented("scale")
	},
}

func init() {
	scaleCmd.Flags().IntVarP(&scaleCount, "count", "n", 1, "number of replicas")
	scaleCmd.Flags().BoolVar(&scaleZero, "zero", false, "suspend (snapshot to NVMe, wake on request)")
	scaleCmd.Flags().StringVarP(&scaleRegion, "region", "r", "", "region to scale in")
}
