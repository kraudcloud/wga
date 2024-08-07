{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}

{{- define "wga.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "wga.fullname" -}}
{{- $name := default .Chart.Name -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "wga.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Endpoint image
*/}}
{{- define "wga.endpointImage" -}}
{{- printf "%s:%s" .Values.endpoint.image.name (coalesce .Values.endpoint.image.tag .Chart.AppVersion .Chart.Version)}}
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
