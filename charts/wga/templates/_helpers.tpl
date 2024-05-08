{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}

{{- define "wga.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "wga.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "wga.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "wga.labels" -}}
helm.sh/chart: {{ include "wga.chart" . }}
{{ include "wga.selectorLabels" . }}
app.kubernetes.io/version: {{ default .Chart.AppVersion | quote }}
version: {{ default .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "wga.selectorLabels" -}}
app.kubernetes.io/name: {{ include "wga.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
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
{{- define "gen.endpoint.privateKey" -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace "endpoint" -}}
{{- if $secret -}}
{{ $secret.data.privateKey | b64dec }}
{{- else -}}
{{ randAlphaNum 32 | b64enc }}
{{- end -}}
{{- end -}}