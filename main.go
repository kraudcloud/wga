package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
	"time"
)

func main() {

	rootCmd := &cobra.Command{}

	serverCmd := &cobra.Command{
		Use:   "ep",
		Short: "run endpoint",
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

func epMain() {

	epConfig, config, err := LoadConfig()
	if err != nil {
		panic(err)
	}

	err = wgInit(epConfig)
	if err != nil {
		panic(err)
	}

	wgSync(config)
	nftSync(config)
	sysctl(config)

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

		_, config, err := LoadConfig()
		if err != nil {
			slog.Error(err.Error())
		} else {

			wgSync(config)
			nftSync(config)
			sysctl(config)
		}

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
