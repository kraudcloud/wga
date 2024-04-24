{{- define "gen.endpoint.privateKey" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace "endpoint" -}}
{{- if $secret -}}
{{ $secret.data.privateKey | b64dec }}
{{- else -}}
{{ randAlphaNum 32 | b64enc }}
{{- end -}}
{{- end -}}
{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "wga.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}
{{/*
Create the name of the service account to use
*/}}
{{- define "wga.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "wga.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}