{{/*
Return the appropriate sidecar images based on k8s version
*/}}
{{- define "csi-isilon.attacherImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "25") -}}
      {{- print "k8s.gcr.io/sig-storage/csi-attacher:v3.5.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.provisionerImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "25") -}}
      {{- print "k8s.gcr.io/sig-storage/csi-provisioner:v3.2.1" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.snapshotterImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "25") -}}
      {{- print "k8s.gcr.io/sig-storage/csi-snapshotter:v6.0.1" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.resizerImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "25") -}}
      {{- print "k8s.gcr.io/sig-storage/csi-resizer:v1.5.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.registrarImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "25") -}}
      {{- print "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.5.1" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- define "csi-isilon.healthmonitorImage" -}}
  {{- if eq .Capabilities.KubeVersion.Major "1" }}
    {{- if and (ge (trimSuffix "+" .Capabilities.KubeVersion.Minor) "21") (le (trimSuffix "+" .Capabilities.KubeVersion.Minor) "25") -}}
      {{- print "gcr.io/k8s-staging-sig-storage/csi-external-health-monitor-controller:v0.6.0" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
