{{- define "gen.endpoint.privateKey" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace "endpoint" -}}
{{- if $secret -}}
{{ $secret.data.privateKey }}
{{- else -}}
{{ randAlphaNum 32 | b64enc | b64enc }}
{{- end -}}
{{- end -}}



{{- $secret := lookup "v1" "ConfigMap" .Release.Namespace "run" -}}
{{ index $secret.data "run.yaml" }}
---
peers: []
{{- end -}}
