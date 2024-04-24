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
		Run: func(cmd *cobra.Command, args []string) {
			epMain()
		},
	}
	rootCmd.AddCommand(serverCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
