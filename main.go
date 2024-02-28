package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

func main() {

	rootCmd := &cobra.Command{}

	serverCmd := &cobra.Command{
		Use: "server",
		Run: func(cmd *cobra.Command, args []string) {
			serverMain()
		},
	}
	rootCmd.AddCommand(serverCmd)

	printCmd := &cobra.Command{
		Use:  "print [clientName]",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printMain(args[0])
		},
	}
	rootCmd.AddCommand(printCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func serverMain() {

	_, err := MakeWgConfig()
	if err != nil {
		slog.Error("MakeWgConfig", "err", err)
	}

	wgUp()
	nftUp()
	sysctl()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	err = watcher.Add("/etc/wga/")
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {

		changed, err := MakeWgConfig()
		if err != nil {
			slog.Error("MakeWgConfig", "err", err)
		}

		if changed {
			// FIXME temp hack because of #1: wg-sync doesnt change routes
			wgUp()
		} else {
			err = wgSync()
			if err != nil {
				slog.Error(err.Error())
				wgUp()
			}
		}

		nftUp()
		sysctl()
		select {
		case <-ticker.C:
			slog.Info("Syncing due to timer")
		case _, ok := <-watcher.Events:
			if !ok {
				panic("fsnotify channel closed")
			}
			slog.Info("Syncing due to fsnotify")
		case err, ok := <-watcher.Errors:
			if !ok {
				panic("fsnotify channel closed")
			}
			panic(err)
		}
	}
}

func wgSync() error {

	stripCmd := exec.Command("wg-quick", "strip", "wg")
	stripCmd.Stderr = os.Stderr
	strippedConfig, err := stripCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Error creating pipe: %v", err)
	}

	if err := stripCmd.Start(); err != nil {
		return fmt.Errorf("Error starting wg-quick strip: %v", err)
	}
	defer stripCmd.Wait()

	syncCmd := exec.Command("wg", "syncconf", "wg", "/dev/stdin")
	syncCmd.Stdin = strippedConfig
	syncCmd.Stdout = os.Stdout
	syncCmd.Stderr = os.Stderr

	if err := syncCmd.Run(); err != nil {
		return fmt.Errorf("Error running wg syncconf: %v", err)
	}

	slog.Info("did wg syncconf")

	return nil
}

func wgUp() {

	exec.Command("wg-quick", "down", "wg").Run()

	cmd := exec.Command("wg-quick", "up", "wg")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		slog.Error("wg-quick up wg", "err", err)
	}
	slog.Info("did wg-quick up wg")
}

func nftUp() {
	cmd := exec.Command("nft", "-f", "/etc/wga/nftables.conf")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("nft -f /etc/wga/nftables.conf", "err", err)
	}
	slog.Info("did nft -f /etc/wga/nftables.conf")
}

func sysctl() {
	cmd := exec.Command("sysctl", "-p", "/etc/wga/sysctl.conf")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("sysctl -p /etc/wga/sysctl.conf", "err", err)
	}
	slog.Info("did sysctl -p /etc/wga/sysctl.conf")
}
