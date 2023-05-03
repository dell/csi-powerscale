{{/*
Return the appropriate sidecar images based on k8s version
*/}}
{{- define "csi-isilon.attacherImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "27") -}}
      {{- print "registry.k8s.io/sig-storage/csi-attacher:v4.2.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.provisionerImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "27") -}}
      {{- print "registry.k8s.io/sig-storage/csi-provisioner:v3.4.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.snapshotterImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "27") -}}
      {{- print "registry.k8s.io/sig-storage/csi-snapshotter:v6.2.1" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.resizerImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "27") -}}
      {{- print "registry.k8s.io/sig-storage/csi-resizer:v1.7.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.registrarImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "27") -}}
      {{- print "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.6.3" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.healthmonitorImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "27") -}}
      {{- print "registry.k8s.io/k8s-staging-sig-storage/csi-external-health-monitor-controller:v0.8.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
Return true if storage capacity tracking is enabled and is supported based on k8s version
*/}}
{{- define "csi-isilon.isStorageCapacitySupported" -}}
{{- if eq .Values.storageCapacity.enabled true -}}
  {{- if and (eq .Capabilities.KubeVersion.Major "1") (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "24") -}}
      {{- true -}}
  {{- end -}}
{{- end -}}
{{- end -}}