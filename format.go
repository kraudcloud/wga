package main

import (
	"io"
	"strings"
	"text/template"
)

const WgFile = `{{- if .ConfigName -}}
# {{.ConfigName }}
{{- end }}
[Interface]
PrivateKey = {{ .PrivateKey }}
Address = {{ .Address }}
{{- if .ListenPort }}
ListenPort = {{ .ListenPort }}
{{- end }}
{{- if .FirewallMark }}
FirewallMark = {{ .FirewallMark }}
{{- end }}

{{- range .Peers }}

[Peer]
Endpoint = {{ .Endpoint }}
PublicKey = {{ .PublicKey }}
{{- if .PresharedKey }}
PresharedKey = {{ .PresharedKey }}
{{- end }}
{{- if .AllowedIPs }}
AllowedIPs = {{ join .AllowedIPs ", " }}
{{- end }}
{{- if .PersistentKeepalive }}
PersistentKeepalive = {{ .PersistentKeepalive }}
{{- end }}
{{- if .LastHandshakeTime }}
LastHandshakeTime = {{ .LastHandshakeTime }}
{{- end }}
{{- if .ReceiveBytes }}
ReceiveBytes = {{ .ReceiveBytes }}
{{- end }}
{{- if .TransmitBytes }}
TransmitBytes = {{ .TransmitBytes }}
{{- end }}{{- end }}
`

type WireguardConfig struct {
	ConfigName   string                `json:"configName"`
	PrivateKey   string                `json:"privateKey"`
	Address      string                `json:"address"`
	ListenPort   int                   `json:"listenPort"`
	FirewallMark int32                 `json:"firewallMark"`
	Peers        []WireguardConfigPeer `json:"peers"`
}

type WireguardConfigPeer struct {
	PublicKey           string   `json:"publicKey"`
	PresharedKey        string   `json:"presharedKey"`
	AllowedIPs          []string `json:"allowedIPs"`
	Endpoint            string   `json:"endpoint"`
	PersistentKeepalive int      `json:"persistentKeepalive"`
	LastHandshakeTime   string   `json:"lastHandshakeTime"`
	ReceiveBytes        int64    `json:"receiveBytes"`
	TransmitBytes       int64    `json:"transmitBytes"`
}

var wgFileTemplate = template.Must(template.New("wg-file").Funcs(template.FuncMap{
	"join": strings.Join,
}).Parse(WgFile))

func Format(w io.Writer, wgConfig WireguardConfig) error {
	return wgFileTemplate.Execute(w, wgConfig)
}
