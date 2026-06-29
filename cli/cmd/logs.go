package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
)

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "Stream or tail logs for a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := args[0]
		fmt.Printf("logs %s env=%s follow=%v tail=%d\n", svc, env, logsFollow, logsTail)
		// TODO: GET /v1/.../services/{svc}/logs?follow=true&tail=N
		// TODO: SSE/WebSocket stream → stdout (lipgloss colouring per level)
		return errNotImplemented("logs")
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "follow log output")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 100, "number of recent lines")
}
