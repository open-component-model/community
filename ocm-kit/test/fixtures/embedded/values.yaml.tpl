{{- $img := index .OCIResources "embedded-image" }}
{{ $img.Host }}/{{ $img.Repository }}:{{ $img.Tag }}@{{ $img.Digest }}