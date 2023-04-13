{{/*
Expand the name of the chart.
*/}}
{{- define "s3gw-cosi.name" -}}
{{- .Chart.Name }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "s3gw-cosi.fullname" -}}
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
{{- define "s3gw-cosi.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "s3gw-cosi.labels" -}}
helm.sh/chart: {{ include "s3gw-cosi.chart" . }}
{{ include "s3gw-cosi.commonSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: container-object-storage-interface
{{- end }}

{{- define "s3gw-cosi.commonSelectorLabels" -}}
app.kubernetes.io/name: {{ include "s3gw-cosi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "s3gw-cosi.selectorLabels" -}}
{{ include "s3gw-cosi.commonSelectorLabels" . }}
app.kubernetes.io/component: driver
{{- end }}

{{/*
Version helpers for the driver image tag
*/}}
{{- define "s3gw-cosi.driverImage" -}}
{{- $defaulttag := printf "%s" "latest" }}
{{- $tag := default $defaulttag .Values.driver.imageTag }}
{{- $name := default "s3gw-cosi-driver" .Values.driver.imageName }}
{{- $registry := default "quay.io/s3gw" .Values.driver.imageRegistry }}
{{- printf "%s/%s:%s" $registry $name $tag }}
{{- end }}

{{/*
Version helpers for the sidecar image tag
*/}}
{{- define "s3gw-cosi.sidecarImage" -}}
{{- $defaulttag := printf "%s" "latest" }}
{{- $tag := default $defaulttag .Values.sidecar.imageTag }}
{{- $name := default "s3gw-cosi-sidecar" .Values.sidecar.imageName }}
{{- $registry := default "quay.io/s3gw" .Values.sidecar.imageRegistry }}
{{- printf "%s/%s:%s" $registry $name $tag }}
{{- end }}
