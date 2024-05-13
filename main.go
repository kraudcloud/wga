package main

import (
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	logLevel := 0
	if lv := os.Getenv("LOG_LEVEL"); lv != "" {
		logLevel, _ = strconv.Atoi(lv)
	}
	vCount := 0

	rootCmd := &cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if vCount > 0 {
				logLevel -= vCount * 4
			}

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
			clientCIDRstr := os.Getenv("WGA_CLIENT_CIDR")
			_, clientCIDR, err := net.ParseCIDR(clientCIDRstr)
			if err != nil {
				slog.Error("cannot parse client cidr", "WGA_CLIENT_CIDR", clientCIDRstr, "err", err.Error())
				os.Exit(1)
			}

			serverAddr := os.Getenv("WGA_SERVER_ADDRESS")
			if serverAddr == "" {
				slog.Error("WGA_SERVER_ADDRESS not set")
				os.Exit(1)
			}

			allowedIPEnv := os.Getenv("WGA_ALLOWED_IPS")
			if allowedIPEnv == "" {
				slog.Error("WGA_ALLOWED_IPS not set")
				os.Exit(1)
			}

			epMain(
				cmd.Context(),
				clientCIDR,
				serverAddr,
				strings.Split(allowedIPEnv, ","),
			)
		},
	}
	rootCmd.AddCommand(serverCmd)

	wgcCmd := &cobra.Command{
		Use:   "clusterclient",
		Short: "run ClusterClient",
		Run: func(cmd *cobra.Command, args []string) {
			wgcMain(
				cmd.Context(),
			)
		},
	}
	rootCmd.AddCommand(wgcCmd)

	rootCmd.AddCommand(peerCmd())

	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
