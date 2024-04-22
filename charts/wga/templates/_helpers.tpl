{{- define "gen.endpoint.privateKey" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace "endpoint" -}}
{{- if $secret -}}
{{ $secret.data.privateKey }}
{{- else -}}
{{ randAlphaNum 32 | b64enc }}
{{- end -}}
{{- end -}}

{{- define "gen.endpoint.peers" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace "endpoint" -}}
{{- if $secret -}}
{{ index $secret.data "run.yaml" }}
{{- else -}}
---
peers: []
{{- end -}}
{{- end -}}
