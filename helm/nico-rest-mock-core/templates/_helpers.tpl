{{/*
Expand the name of the chart.
*/}}
{{- define "nico-rest-mock-core.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "nico-rest-mock-core.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "nico-rest-mock-core.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nico-rest-mock-core.labels" -}}
helm.sh/chart: {{ include "nico-rest-mock-core.chart" . }}
{{ include "nico-rest-mock-core.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "nico-rest-mock-core.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nico-rest-mock-core.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "nico-rest-mock-core.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "nico-rest-mock-core.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Default libvirt root volume size in GiB (used when OS image capacity is unknown).
*/}}
{{- define "nico-rest-mock-core.libvirtVolumeGiB" -}}
{{- default 20 .Values.libvirt.volumeGiB }}
{{- end }}
