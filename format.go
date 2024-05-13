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
DNS = {{ joinIPs .DNS ", " }}

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

const WgcFile = `apiVersion: wga.kraudcloud.com/v1beta
kind: WireguardClusterClient
metadata:
  name: {{.Name}}
spec:
  address: {{ .Address }}
  privateKeySecretRef:
    value: {{ .PrivateKey }}
{{- range .Peers }}
  server:
    endpoint: {{ .Endpoint }}
    publicKey: {{ .PublicKey }}
    {{- if validKey .PresharedKey }}
    preSharedKey: {{ .PresharedKey }}
    {{- end }}
  routes:
  {{- range .AllowedIPs}}
    - {{.}}
  {{- end }}
  {{- if .PersistentKeepaliveInterval }}
  persistentKeepalive: {{ seconds .PersistentKeepaliveInterval }}
  {{- end }}
{{- end }}

`

var funcs = template.FuncMap{
	"joinIPs": func(ips []net.IP, sep string) string {
		strs := make([]string, 0, len(ips))
		for _, ip := range ips {
			strs = append(strs, ip.String())
		}
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
var wgcFileTemplate = template.Must(template.New("wgc-yaml").Funcs(funcs).Parse(WgcFile))

type ConfigFile struct {
	Address *net.IPNet
	DNS     []net.IP
	wgtypes.Device
	Name string
}

func Format(w io.Writer, wgConfig ConfigFile) error {
	if wgConfig.Name != "" {
		return wgcFileTemplate.Execute(w, wgConfig)
	} else {
		return wgFileTemplate.Execute(w, wgConfig)
	}
}
