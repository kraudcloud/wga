package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	t.Parallel()
	type args struct {
		wgConfig WireguardConfig
	}

	tests := []struct {
		name    string
		args    args
		wantW   string
		wantErr bool
	}{
		{
			name: "Valid WireGuard config",
			args: args{
				wgConfig: WireguardConfig{
					ConfigName: "Bocchi (Phone)",
					PrivateKey: "abcdefghijklmnopqrstuvwxyz0123456789=",
					ListenPort: 51820,
					Peers: []WireguardConfigPeer{
						{
							PublicKey:  "qwertyuiopasdfghjklzxcvbnm1234567890=",
							AllowedIPs: []string{"10.0.0.1/32"},
							Endpoint:   "example.com:51820",
						},
					},
				},
			},
			wantW: `# Bocchi (Phone)
[Interface]
 PrivateKey = abcdefghijklmnopqrstuvwxyz0123456789=
 ListenPort = 51820
 
 [Peer]
 PublicKey = qwertyuiopasdfghjklzxcvbnm1234567890=
 AllowedIPs = 10.0.0.1/32
 Endpoint = example.com:51820
 `,
			wantErr: false,
		},
		{
			name: "WireGuard config with optional fields",
			args: args{
				wgConfig: WireguardConfig{
					PrivateKey:   "abcdefghijklmnopqrstuvwxyz0123456789=",
					ListenPort:   51820,
					FirewallMark: 1234,
					Peers: []WireguardConfigPeer{
						{
							PublicKey:           "qwertyuiopasdfghjklzxcvbnm1234567890=",
							PresharedKey:        "0123456789abcdefghijklmnopqrstuvwxyz=",
							AllowedIPs:          []string{"10.0.0.1/32", "10.0.0.2/32"},
							Endpoint:            "example.com:51820",
							PersistentKeepalive: 25,
							LastHandshakeTime:   "2023-05-30 12:34:56",
							ReceiveBytes:        1024,
							TransmitBytes:       2048,
						},
					},
				},
			},
			wantW: `[Interface]
 PrivateKey = abcdefghijklmnopqrstuvwxyz0123456789=
 ListenPort = 51820
 FirewallMark = 1234
 
 [Peer]
 PublicKey = qwertyuiopasdfghjklzxcvbnm1234567890=
 PresharedKey = 0123456789abcdefghijklmnopqrstuvwxyz=
 AllowedIPs = 10.0.0.1/32, 10.0.0.2/32
 Endpoint = example.com:51820
 PersistentKeepalive = 25
 LastHandshakeTime = 2023-05-30 12:34:56
 ReceiveBytes = 1024
 TransmitBytes = 2048
 `,
			wantErr: false,
		},
		{
			name: "Empty WireGuard config",
			args: args{
				wgConfig: WireguardConfig{},
			},
			wantW: `[Interface]
 PrivateKey = 
 ListenPort = 0`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			if err := Format(w, tt.args.wgConfig); (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			t.Log(w.String())
			want := simplify(tt.wantW)
			gotW := simplify(w.String())
			if gotW != want {
				t.Errorf("Format() = %q, want %q", gotW, want)
			}
		})
	}
}

func simplify(s string) string {
	return strings.NewReplacer("\n", "", " ", "").Replace(s)
}
