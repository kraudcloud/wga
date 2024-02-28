package main

import (
	"bytes"
	"fmt"
	"github.com/mdp/qrterminal/v3"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log/slog"
	"os"
	"strings"
)

func printMain(clientName string) {

	config, err := LoadConfig()
	if err != nil {
		slog.Error("LoadConfig", "err", err)
		return
	}

	srvprivkey, err := wgtypes.ParseKey(config.Server.PrivateKey)
	if err != nil {
		slog.Error("base64 decode of server private key failed", "err", err)
		return
	}

	srcpubkey := srvprivkey.PublicKey().String()

	for _, peer := range config.Peers {
		if peer.Name == clientName {

			var b bytes.Buffer
			var o = &b

			fmt.Fprintf(o, "### %s\n", peer.Name)
			fmt.Fprintf(o, "[Interface] \n")
			fmt.Fprintf(o, "PrivateKey = %s\n", peer.PrivateKey)
			fmt.Fprintf(o, "Address = %s\n", strings.Join(peer.IPs, ","))
			fmt.Fprintf(o, "\n")

			fmt.Fprintf(o, "[Peer] \n")
			fmt.Fprintf(o, "Endpoint = %s\n", config.Print.PublicEndpoint)
			fmt.Fprintf(o, "PublicKey = %s\n", srcpubkey)
			fmt.Fprintf(o, "AllowedIPs = %s\n", strings.Join(peer.Routes, ","))
			fmt.Fprintf(o, "\n")
			fmt.Fprintf(o, "\n")

			fmt.Println(b.String())

			qrterminal.Generate(b.String(), qrterminal.L, os.Stderr)

			return
		}
	}

}
