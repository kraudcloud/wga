package main

import (
	"io"
	"net"
	"strings"
	"text/template"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const WgFile = `{{- if .Name -}}
# {{.Name }}
{{- end }}
[Interface]
PrivateKey = {{ .PrivateKey }}
Address = {{ .Address }}

{{- range .Peers }}

[Peer]
Endpoint = {{ .Endpoint }}
PublicKey = {{ .PublicKey }}
{{- if validKey .PresharedKey }}
PresharedKey = {{ .PresharedKey }}
{{- end }}
{{- if .AllowedIPs }}
AllowedIPs = {{ joinIPs .AllowedIPs ", " }}
{{- end }}
{{- if .PersistentKeepaliveInterval }}
PersistentKeepalive = {{ .PersistentKeepaliveInterval }}
{{- end }}{{- end }}
`

var wgFileTemplate = template.Must(template.New("wg-file").Funcs(template.FuncMap{
	"joinIPs": func(ips []net.IPNet, sep string) string {
		strs := make([]string, 0, len(ips))
		for _, ip := range ips {
			strs = append(strs, ip.String())
		}
		return strings.Join(strs, sep)
	},
	"validKey": func(k wgtypes.Key) bool {
		return k != wgtypes.Key{}
	},
}).Parse(WgFile))

type ConfigFile struct {
	Address *net.IPNet
	wgtypes.Device
}

func Format(w io.Writer, wgConfig ConfigFile) error {
	return wgFileTemplate.Execute(w, wgConfig)
}
