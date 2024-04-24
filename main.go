package main

import (
	"github.com/spf13/cobra"
	"log/slog"
	"os"
)

func main() {

	rootCmd := &cobra.Command{}

	serverCmd := &cobra.Command{
		Use:   "ep [name]",
		Short: "run named WireguardAccessEndpoint",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			epMain(args[0])
		},
	}
	rootCmd.AddCommand(serverCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
