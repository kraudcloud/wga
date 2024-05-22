package main

import (
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/kraudcloud/wga/operator"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
			_, peersNet, err := net.ParseCIDR(clientCIDRstr)
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

			allowedIPs := strings.Split(allowedIPEnv, ",")
			serviceNets := []net.IPNet{}
			for _, ip := range allowedIPs {
				_, ipnet, err := net.ParseCIDR(ip)
				if err != nil {
					slog.Error("cannot parse allowed ip", "allowedIP", ip, "err", err.Error())
				}
				serviceNets = append(serviceNets, *ipnet)
			}

			operator.RunWGA(cmd.Context(), clientConfig(), serviceNets, []net.IPNet{*peersNet}, serverAddr)
		},
	}
	rootCmd.AddCommand(serverCmd)

	wgcCmd := &cobra.Command{
		Use:   "clusterclient",
		Short: "run ClusterClient",
		Run: func(cmd *cobra.Command, args []string) {
			operator.RunWGC(cmd.Context(), clientConfig())
		},
	}
	rootCmd.AddCommand(wgcCmd)

	rootCmd.AddCommand(peerCmd())

	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// clientConfig loads the config either from kubeconfig or falls back to the cluster
// the k8s client has a similar function but it logs stuff when trying to fallback.
func clientConfig() *rest.Config {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		c, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}, &clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			slog.Error("cannot load kubeconfig", "kubeconfig", kubeconfig, "err", err.Error())
			os.Exit(1)
		}

		return c
	}

	c, err := rest.InClusterConfig()
	if err != nil {
		slog.Error("cannot load in-cluster config", "err", err.Error())
		os.Exit(1)
	}

	return c
}
