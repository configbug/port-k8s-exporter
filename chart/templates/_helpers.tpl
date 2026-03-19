{{/*
Expand the name of the chart.
*/}}
{{- define "port-k8s-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "port-k8s-exporter.fullname" -}}
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
{{- define "port-k8s-exporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "port-k8s-exporter.labels" -}}
helm.sh/chart: {{ include "port-k8s-exporter.chart" . }}
{{ include "port-k8s-exporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "port-k8s-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "port-k8s-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
ServiceAccount name
*/}}
{{- define "port-k8s-exporter.serviceAccountName" -}}
{{- if .Values.serviceAccount.nameOverride }}
{{- .Values.serviceAccount.nameOverride }}
{{- else }}
{{- include "port-k8s-exporter.fullname" . }}
{{- end }}
{{- end }}

{{/*
Lookup cluster entry from inventory.
Fails if clusterName is not in clusterInventory.
*/}}
{{- define "port-k8s-exporter.clusterEntry" -}}
{{- $name := required "clusterName is required (e.g., aksinfrdesa001)" .Values.clusterName }}
{{- if not (hasKey .Values.clusterInventory $name) }}
{{- fail (printf "cluster '%s' not found in clusterInventory. Add it to the values file." $name) }}
{{- end }}
{{- index .Values.clusterInventory $name | toYaml }}
{{- end }}

{{/*
Resolve the cluster group from inventory.
*/}}
{{- define "port-k8s-exporter.clusterGroup" -}}
{{- $name := required "clusterName is required (e.g., aksinfrdesa001)" .Values.clusterName }}
{{- if not (hasKey .Values.clusterInventory $name) }}
{{- fail (printf "cluster '%s' not found in clusterInventory. Add it to the values file." $name) }}
{{- end }}
{{- $entry := index .Values.clusterInventory $name }}
{{- $entry.group | toString }}
{{- end }}

{{/*
Resolve the cluster size from inventory.
*/}}
{{- define "port-k8s-exporter.clusterSize" -}}
{{- $name := required "clusterName is required (e.g., aksinfrdesa001)" .Values.clusterName }}
{{- if not (hasKey .Values.clusterInventory $name) }}
{{- fail (printf "cluster '%s' not found in clusterInventory. Add it to the values file." $name) }}
{{- end }}
{{- $entry := index .Values.clusterInventory $name }}
{{- $entry.size }}
{{- end }}

{{/*
Resolve the webhook URL for the current cluster.
Looks up group from inventory, then resolves URL from webhookUrls.
*/}}
{{- define "port-k8s-exporter.webhookUrl" -}}
{{- $group := include "port-k8s-exporter.clusterGroup" . }}
{{- if not (hasKey .Values.webhookUrls $group) }}
{{- fail (printf "clusterGroup '%s' not found in webhookUrls. Valid groups: %s" $group (keys .Values.webhookUrls | sortAlpha | join ", ")) }}
{{- end }}
{{- index .Values.webhookUrls $group }}
{{- end }}

{{/*
Resolve resource limits based on cluster size from inventory.
*/}}
{{- define "port-k8s-exporter.resources" -}}
{{- $size := include "port-k8s-exporter.clusterSize" . }}
{{- if not (hasKey .Values.sizeProfiles $size) }}
{{- fail (printf "clusterSize '%s' not found in sizeProfiles. Valid sizes: %s" $size (keys .Values.sizeProfiles | sortAlpha | join ", ")) }}
{{- end }}
{{- toYaml (index .Values.sizeProfiles $size) }}
{{- end }}
