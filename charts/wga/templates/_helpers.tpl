{{- define "gen.endpoint.privateKey" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace "endpoint" -}}
{{- if $secret -}}
{{ $secret.data.privateKey | b64dec }}
{{- else -}}
{{ randAlphaNum 32 | b64enc }}
{{- end -}}
{{- end -}}
