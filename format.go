package main

import (
	"io"
	"net"
	"strings"
	"text/template"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const WgFile = `{{- if .Name -}}
# {{.Name }}
{{- end }}
[Interface]
PrivateKey = {{ .PrivateKey }}
Address = {{ .Address }}
DNS = {{ join .DNS ", " }}

{{- range .Peers }}

[Peer]
Endpoint = {{ .Endpoint }}
PublicKey = {{ .PublicKey }}
{{- if validKey .PresharedKey }}
PresharedKey = {{ .PresharedKey }}
{{- end }}
{{- if .AllowedIPs }}
AllowedIPs = {{ joinIPNets .AllowedIPs ", " }}
{{- end }}
{{- if .PersistentKeepaliveInterval }}
PersistentKeepalive = {{ seconds .PersistentKeepaliveInterval }}
{{- end }}{{- end }}
`

var funcs = template.FuncMap{
	"join": func(strs []string, sep string) string {
		return strings.Join(strs, sep)
	},
	"joinIPNets": func(ips []net.IPNet, sep string) string {
		strs := make([]string, 0, len(ips))
		for _, ip := range ips {
			strs = append(strs, ip.String())
		}
		return strings.Join(strs, sep)
	},
	"validKey": func(k wgtypes.Key) bool {
		return k != wgtypes.Key{}
	},
	"seconds": func(d time.Duration) int {
		return int(d.Seconds())
	},
}

var wgFileTemplate = template.Must(template.New("wg-file").Funcs(funcs).Parse(WgFile))

type ConfigFile struct {
	Address *net.IPNet
	DNS     []string
	wgtypes.Device
	Name string
}

func Format(w io.Writer, wgConfig ConfigFile) error {
	return wgFileTemplate.Execute(w, wgConfig)
}
