{{- define "secure-inference.componentConfig" -}}
type: {{ .type | default "" | quote }}
{{- if .parameters }}
parameters:
{{- range $k, $v := .parameters }}
  {{ $k }}: {{ $v | quote }}
{{- end }}
{{- end }}
{{- end -}}
