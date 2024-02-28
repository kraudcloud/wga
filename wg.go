package main

import (
	"bytes"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log/slog"
	"os"
	"strings"
)

func MakeWgConfig() (changed bool, err error) {

	config, err := LoadConfig()
	if err != nil {
		return false, err
	}

	os.MkdirAll("/etc/wireguard", 0700)

	var b bytes.Buffer
	var o = &b

	fmt.Fprintf(o, "[Interface]\n")
	fmt.Fprintf(o, "PrivateKey = %s\n", config.Server.PrivateKey)
	fmt.Fprintf(o, "ListenPort = 51820\n")
	fmt.Fprintf(o, "Address = %s\n", strings.Join(config.Server.IPs, ","))
	fmt.Fprintf(o, "\n")

	for _, p := range config.Peers {

		privkey, err := wgtypes.ParseKey(p.PrivateKey)
		if err != nil {
			slog.Error("base64 decode of peer "+p.Name+" private key failed", "err", err)
			continue
		}

		pubkey64 := privkey.PublicKey().String()

		fmt.Fprintf(o, "[Peer]\n")
		fmt.Fprintf(o, "PublicKey = %s\n", pubkey64)
		fmt.Fprintf(o, "AllowedIPs = %s\n", strings.Join(p.IPs, ","))
		fmt.Fprintf(o, "\n")
	}

	existing, err := os.ReadFile("/etc/wireguard/wg.conf")
	if err != nil {
		existing = []byte{}
	}

	if !bytes.Equal(b.Bytes(), existing) {

		// Config is different, write new config
		err = os.WriteFile("/etc/wireguard/wg.conf", b.Bytes(), 0600)
		if err != nil {
			return false, err
		}

		changed = true
	}

	return changed, nil
}
