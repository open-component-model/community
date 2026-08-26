package helmvalues

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	ociaccessv1 "ocm.software/open-component-model/bindings/go/oci/spec/access/v1"
	"ocm.software/open-component-model/bindings/go/runtime"
)

const configWithHelmCredentials = `
type: generic.config.ocm.software/v1
configurations:
  - type: credentials.config.ocm.software
    consumers:
      - identities:
          - type: HelmChartRepository
            hostname: example.org 
        credentials:
          - type: HelmHTTPCredentials/v1
            username: anything
            password: anything
`

// TestRender tests the Render function with various template scenarios
func TestRender(t *testing.T) {
	tests := []struct {
		name      string
		template  *HelmValuesTemplate
		input     *RenderingInput
		options   []RenderOption
		wantMatch string
		wantErr   bool
	}{
		{
			name: "simple template with resources",
			template: &HelmValuesTemplate{
				ResourceName:    "test-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `image: {{ index .OCIResources "app" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{
					"app": mkImageRef("myregistry.com/myapp:1.0.0"),
				},
			},
			wantMatch: "image: myregistry.com/myapp:1.0.0",
			wantErr:   false,
		},
		{
			name:     "nil template",
			template: nil,
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
			},
			wantErr: true,
		},
		{
			name: "nil input",
			template: &HelmValuesTemplate{
				ResourceName:    "test",
				ResourceVersion: "1.0.0",
				TemplateContent: "test",
			},
			input:   nil,
			wantErr: true,
		},
		{
			name: "invalid template syntax",
			template: &HelmValuesTemplate{
				ResourceName:    "invalid",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{.OCIResources | invalid_func}}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
			},
			wantErr: true,
		},
		{
			name: "template with conditional logic",
			template: &HelmValuesTemplate{
				ResourceName:    "conditional",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{- if index .OCIResources "app" -}}app exists{{- else -}}app missing{{- end -}}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{
					"app": mkImageRef("present"),
				},
			},
			wantMatch: "app exists",
			wantErr:   false,
		},
		{
			name: "template with range over resources",
			template: &HelmValuesTemplate{
				ResourceName:    "range-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{- range $k, $v := .OCIResources }}{{ $k }}: {{ $v }}
{{- end }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{
					"app1": mkImageRef("image1"),
					"app2": mkImageRef("image2"),
				},
			},
			wantMatch: "app1: image1",
			wantErr:   false,
		},
		{
			name: "template with invalid yaml and validation disabled",
			template: &HelmValuesTemplate{
				ResourceName:    "invalid-yaml-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `{key1: value1, key2: : value2}`,
			},
			input:   &RenderingInput{},
			wantErr: false,
		},
		{
			name: "template with invalid yaml and validation enabled",
			template: &HelmValuesTemplate{
				ResourceName:    "invalid-yaml-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `{key1: value1, key2: : value2}`,
			},
			input:   &RenderingInput{},
			options: []RenderOption{WithYAMLValidation()},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.template, tt.input, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if tt.wantMatch != "" && !strings.Contains(got, tt.wantMatch) {
				t.Errorf("Render() output doesn't contain expected text.\nGot: %s\nExpected to contain: %s", got, tt.wantMatch)
			}
		})
	}
}

// TestPullSecretsResolve tests the Resolve method with OCI refs and raw registries
func TestPullSecretsResolve(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		secrets PullSecrets
		want    string
	}{
		{
			name:    "full ref matches Host/Repository",
			ref:     "ghcr.io/org/myapp:v1.0.0",
			secrets: PullSecrets{"ghcr.io/org/myapp": "repo-cred"},
			want:    "repo-cred",
		},
		{
			name:    "full ref matches host only",
			ref:     "ghcr.io/org/myapp:v1.0.0",
			secrets: PullSecrets{"ghcr.io": "org-cred"},
			want:    "org-cred",
		},
		{
			name: "Host/Repository takes priority over host",
			ref:  "ghcr.io/org/myapp:v1.0.0",
			secrets: PullSecrets{
				"ghcr.io/org/myapp": "repo-cred",
				"ghcr.io":           "org-cred",
			},
			want: "repo-cred",
		},
		{
			name:    "ref with nested path matches correctly",
			ref:     "registry.example.com/team/service/sub:v2",
			secrets: PullSecrets{"registry.example.com/team/service/sub": "nested-cred"},
			want:    "nested-cred",
		},
		{
			name:    "ref matches intermediate org path, not just host",
			ref:     "docker.io/team-a/my-repo:latest",
			secrets: PullSecrets{"docker.io/team-a": "team-a-secret"},
			want:    "team-a-secret",
		},
		{
			name: "most specific match wins among path hierarchy",
			ref:  "docker.io/team-a/my-repo:latest",
			secrets: PullSecrets{
				"docker.io/team-a/my-repo": "repo-secret",
				"docker.io/team-a":         "org-secret",
				"docker.io":                "global-secret",
			},
			want: "repo-secret",
		},
		{
			name:    "intermediate org resolves independently per organization",
			ref:     "docker.io/team-a/svc:latest",
			secrets: PullSecrets{"docker.io/team-b": "team-b-secret"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.secrets.Resolve(tt.ref)
			if got != tt.want {
				t.Errorf("PullSecrets.Resolve(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

// TestPullSecretsGet tests the PullSecrets type directly
func TestPullSecretsGet(t *testing.T) {
	tests := []struct {
		name     string
		secrets  PullSecrets
		registry string
		want     string
	}{
		{
			name:     "known registry returns secret",
			secrets:  PullSecrets{"docker.io": "regcred", "ghcr.io": "ghcr-cred"},
			registry: "docker.io",
			want:     "regcred",
		},
		{
			name:     "unknown registry returns empty string",
			secrets:  PullSecrets{"docker.io": "regcred"},
			registry: "unknown.registry.io",
			want:     "",
		},
		{
			name:     "nil PullSecrets returns empty string",
			secrets:  nil,
			registry: "docker.io",
			want:     "",
		},
		{
			name:     "empty PullSecrets returns empty string",
			secrets:  PullSecrets{},
			registry: "docker.io",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.secrets.Get(tt.registry)
			if got != tt.want {
				t.Errorf("PullSecrets.Get(%q) = %q, want %q", tt.registry, got, tt.want)
			}
		})
	}
}

// TestRenderPullSecretFor tests the pullSecretFor template function via Render
func TestRenderPullSecretFor(t *testing.T) {
	tests := []struct {
		name     string
		template *HelmValuesTemplate
		input    *RenderingInput
		want     string
		wantErr  bool
	}{
		{
			name: "pullSecretFor with matching registry",
			template: &HelmValuesTemplate{
				ResourceName:    "pull-secret-test",
				ResourceVersion: "1.0.0",
				TemplateContent: `secret: {{ pullSecretFor "docker.io" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
				PullSecrets: PullSecrets{
					"docker.io": "regcred",
				},
			},
			want:    "secret: regcred",
			wantErr: false,
		},
		{
			name: "pullSecretFor with non-matching registry",
			template: &HelmValuesTemplate{
				ResourceName:    "pull-secret-no-match",
				ResourceVersion: "1.0.0",
				TemplateContent: `secret: {{ pullSecretFor "unknown.io" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
				PullSecrets: PullSecrets{
					"docker.io": "regcred",
				},
			},
			want:    "secret: ",
			wantErr: false,
		},
		{
			name: "pullSecretFor with ref",
			template: &HelmValuesTemplate{
				ResourceName:    "pull-secret-ref",
				ResourceVersion: "1.0.0",
				TemplateContent: `secret: {{ pullSecretFor "registry.example.com/repo/image:tag" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
				PullSecrets: PullSecrets{
					"registry.example.com": "example-cred",
				},
			},
			want:    "secret: example-cred",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.template, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mkImageRef(ref string) ImageReference {
	parsed, err := ParseOCIRef(ref)
	if err != nil {
		panic(err)
	}
	return parsed
}

// mkDescriptor builds an in-memory descriptor with the given component
// name/version and resources for exercising the find/fetch/get functions.
func mkDescriptor(name, version string, resources ...descriptor.Resource) *descriptor.Descriptor {
	d := &descriptor.Descriptor{}
	d.Component.Name = name
	d.Component.Version = version
	d.Component.Resources = resources
	return d
}

// TestGetFirstHelmValuesTemplate verifies FindFirst+Fetch returns the labeled
// template's content downloaded via the repository.
func TestGetFirstHelmValuesTemplate(t *testing.T) {
	res := resWithLabel("values",
		descriptor.Label{Name: LabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)},
	)
	res.Version = "2.0.0"
	desc := mkDescriptor("acme.org/app", "1.0.0", res)

	repo := &FakeRepository{
		Descriptor: desc,
		Blobs:      map[string][]byte{"values": []byte("replicas: 3")},
	}

	tmpl, err := GetFirstHelmValuesTemplate(context.Background(), repo, desc)
	if err != nil {
		t.Fatalf("GetFirstHelmValuesTemplate() error = %v", err)
	}
	if tmpl.ResourceName != "values" {
		t.Errorf("ResourceName = %q, want %q", tmpl.ResourceName, "values")
	}
	if tmpl.ResourceVersion != "2.0.0" {
		t.Errorf("ResourceVersion = %q, want %q", tmpl.ResourceVersion, "2.0.0")
	}
	if tmpl.TemplateContent != "replicas: 3" {
		t.Errorf("TemplateContent = %q, want %q", tmpl.TemplateContent, "replicas: 3")
	}
}

// TestGetFirstHelmValuesTemplateNotFound verifies ErrNotFound when no resource
// carries a helm-values label.
func TestGetFirstHelmValuesTemplateNotFound(t *testing.T) {
	desc := mkDescriptor("acme.org/app", "1.0.0", resWithLabel("plain"))
	repo := &FakeRepository{Descriptor: desc, Blobs: map[string][]byte{}}

	_, err := GetFirstHelmValuesTemplate(context.Background(), repo, desc)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// TestGetHelmValuesTemplate verifies matching by chart name and ErrNotFound for
// a non-matching chart.
func TestGetHelmValuesTemplate(t *testing.T) {
	values := resWithLabel("values",
		descriptor.Label{Name: LabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)},
	)
	other := resWithLabel("other",
		descriptor.Label{Name: LabelHelmValuesFor, Value: json.RawMessage(`"otherchart"`)},
	)
	desc := mkDescriptor("acme.org/app", "1.0.0", other, values)

	repo := &FakeRepository{
		Descriptor: desc,
		Blobs: map[string][]byte{
			"values": []byte("for: mychart"),
			"other":  []byte("for: otherchart"),
		},
	}

	tmpl, err := GetHelmValuesTemplate(context.Background(), repo, desc, "mychart")
	if err != nil {
		t.Fatalf("GetHelmValuesTemplate() error = %v", err)
	}
	if tmpl.ResourceName != "values" {
		t.Errorf("ResourceName = %q, want %q", tmpl.ResourceName, "values")
	}
	if tmpl.TemplateContent != "for: mychart" {
		t.Errorf("TemplateContent = %q, want %q", tmpl.TemplateContent, "for: mychart")
	}

	_, err = GetHelmValuesTemplate(context.Background(), repo, desc, "no-such-chart")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// TestGetRenderingInput verifies OCI resources are parsed into OCIResources and
// the component is attached.
func TestGetRenderingInput(t *testing.T) {
	ociRes := descriptor.Resource{
		ElementMeta: descriptor.ElementMeta{
			ObjectMeta: descriptor.ObjectMeta{Name: "app"},
		},
		Access: &ociaccessv1.OCIImage{ImageReference: "ghcr.io/acme/app:v1"},
	}
	// A relative (LocalBlob) resource with only a host-less ReferenceName must be
	// reconstructed into a host-qualified reference using the repo base URL.
	relRes := descriptor.Resource{
		ElementMeta: descriptor.ElementMeta{
			ObjectMeta: descriptor.ObjectMeta{Name: "rel"},
		},
		Access: &v2.LocalBlob{
			Type:          runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
			ReferenceName: "opendefensecloud/arc-apiserver:v0.2.0",
		},
	}
	// A non-OCI resource must be skipped.
	plainRes := resWithLabel("values")

	desc := mkDescriptor("acme.org/app", "1.0.0", ociRes, relRes, plainRes)

	const repoBaseURL = "127.0.0.1:5000/my-components"
	input, err := GetRenderingInput(desc, repoBaseURL)
	if err != nil {
		t.Fatalf("GetRenderingInput() error = %v", err)
	}
	if input.Component != &desc.Component {
		t.Errorf("Component = %p, want %p", input.Component, &desc.Component)
	}
	if len(input.OCIResources) != 2 {
		t.Fatalf("len(OCIResources) = %d, want 2", len(input.OCIResources))
	}
	got, ok := input.OCIResources["app"]
	if !ok {
		t.Fatalf("OCIResources missing key %q", "app")
	}
	want := ImageReference{Host: "ghcr.io", Repository: "acme/app", Tag: "v1"}
	if got != want {
		t.Errorf("OCIResources[app] = %#v, want %#v", got, want)
	}
	// The relative resource is now host-qualified via the base URL.
	gotRel, ok := input.OCIResources["rel"]
	if !ok {
		t.Fatalf("OCIResources missing key %q", "rel")
	}
	wantRel := ImageReference{Host: "127.0.0.1:5000", Repository: "my-components/opendefensecloud/arc-apiserver", Tag: "v0.2.0"}
	if gotRel != wantRel {
		t.Errorf("OCIResources[rel] = %#v, want %#v", gotRel, wantRel)
	}
}

// Test if a LocalBlob (which is an OCI image) without a ReferenceName,
// can be resolved as described here: https://ocm.software/docs/tutorials/working-with-oci/
func TestGetRenderingInput_NativeOCIAccess(t *testing.T) {
	res := descriptor.Resource{
		ElementMeta: descriptor.ElementMeta{
			ObjectMeta: descriptor.ObjectMeta{Name: "image", Version: "1.0.1"},
		},
		Type: "ociArtifact",
		Access: &v2.LocalBlob{
			LocalReference: "sha256:0e54ff336823b4f59c54d4b9965d7c53d3cf15c230d0f5e5924d9ee328e9bc77",
			MediaType:      "application/vnd.oci.image.manifest.v1+json",
			Type:           runtime.NewVersionedType(v2.LocalBlobAccessType, v2.LocalBlobAccessTypeVersion),
		},
	}

	desc := mkDescriptor("acme.org/app", "1.0.0", res)

	const repoBaseURL = "127.0.0.1:5000/my-components"
	input, err := GetRenderingInput(desc, repoBaseURL)

	if assert.NoError(t, err) {
		assert.Equal(t, map[string]ImageReference{
			"image": {
				Host:       "127.0.0.1:5000",
				Repository: "my-components/component-descriptors/acme.org/app",
				Tag:        "1.0.1",
				Digest:     "sha256:0e54ff336823b4f59c54d4b9965d7c53d3cf15c230d0f5e5924d9ee328e9bc77",
			},
		}, input.OCIResources)
	}
}
