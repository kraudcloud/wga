package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

func main() {
	wgUp()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	err = watcher.Add("/etc/wireguard/")
	if err != nil {
		panic(err)
	}

	for {
		if err := wgSync(); err != nil {
			slog.Error(err.Error())
			wgUp()
		}
		select {
		case <-time.After(30 * time.Second):
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
}
