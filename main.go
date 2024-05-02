package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func main() {
	logLevel := 4
	if lv := os.Getenv("LOG_LEVEL"); lv != "" {
		logLevel, _ = strconv.Atoi(lv)
	}
	vCount := 0

	rootCmd := &cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if vCount > 0 {
				logLevel -= vCount * 4
			}

			fmt.Println("logLevel", logLevel)

			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.Level(logLevel),
			})))
		},
	}
	rootCmd.PersistentFlags().CountVarP(&vCount, "verbose", "v", "log level")

	serverCmd := &cobra.Command{
		Use:   "ep [name]",
		Short: "run named WireguardAccessEndpoint",
		Run: func(cmd *cobra.Command, args []string) {
			epMain()
		},
	}
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(peerCmd())

	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
