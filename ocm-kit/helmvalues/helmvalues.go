package helmvalues

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"sigs.k8s.io/yaml"
)

// ErrNotFound is returned when a requested Helm values template is not found
var ErrNotFound = errors.New("not found")

// HelmValuesTemplate represents a Helm values template found in an OCM component.
// It contains the template content along with metadata about its resource.
type HelmValuesTemplate struct {
	ResourceName    string
	ResourceVersion string
	TemplateContent string
}

// PullSecrets is a collection of registry-to-secret mappings for pull secrets.
type PullSecrets map[string]string

// Get safely returns the secret name for a registry, falling back to the empty string.
func (p PullSecrets) Get(registry string) string {
	if p == nil {
		return ""
	}
	return p[registry]
}

// Resolve parses ref as an OCI reference and walks the path from most specific
// to least specific (Host/path/to/image -> Host/path/to -> … -> Host).
// If no match is found this way it tries to lookup the raw string.
func (p PullSecrets) Resolve(ref string) string {
	if !strings.Contains(ref, "/") {
		return p.Get(ref)
	}
	parsed, err := ParseOCIRef(ref)
	if err != nil || parsed.Repository == "" {
		return p.Get(ref)
	}
	parts := strings.Split(parsed.Repository, "/")
	for i := len(parts); i >= 0; i-- {
		key := parsed.Host
		if i > 0 {
			key += "/" + strings.Join(parts[:i], "/")
		}
		if secret := p.Get(key); secret != "" {
			return secret
		}
	}
	return p.Get(ref)
}

// RenderingInput contains all the data needed to render a Helm values template.
// It provides access to component resources and the component descriptor for template processing.
type RenderingInput struct {
	OCIResources map[string]ImageReference
	Component    *descriptor.Component
	PullSecrets  PullSecrets
}

// RenderOption is a functional option for configuring Render behavior
type RenderOption func(*renderConfig)

// renderConfig holds configuration for the Render function
type renderConfig struct {
	validateYAML bool
}

// WithYAMLValidation enables YAML validation of the rendered output
func WithYAMLValidation() RenderOption {
	return func(rc *renderConfig) {
		rc.validateYAML = true
	}
}

// FindHelmValuesTemplate returns the resource in desc carrying a helm-values
// label whose value matches chart, or ErrNotFound if no such resource exists.
func FindHelmValuesTemplate(desc *descriptor.Descriptor, chart string) (*descriptor.Resource, error) {
	for i := range desc.Component.Resources {
		if matchesHelmValuesLabel(desc.Component.Resources[i], chart) {
			res := desc.Component.Resources[i]
			return &res, nil
		}
	}

	return nil, ErrNotFound
}

// FindFirstHelmValuesTemplate returns the first resource in desc carrying any
// helm-values label, regardless of value, or ErrNotFound if none exists.
func FindFirstHelmValuesTemplate(desc *descriptor.Descriptor) (*descriptor.Resource, error) {
	for i := range desc.Component.Resources {
		if hasAnyHelmValuesLabel(desc.Component.Resources[i]) {
			res := desc.Component.Resources[i]
			return &res, nil
		}
	}

	return nil, ErrNotFound
}

// FetchHelmValuesTemplate downloads the content of the template resource res
// from repo and returns it as a HelmValuesTemplate.
func FetchHelmValuesTemplate(ctx context.Context, repo Repository, desc *descriptor.Descriptor, res *descriptor.Resource) (*HelmValuesTemplate, error) {
	data, err := repo.ResourceBytes(ctx, desc.Component.Name, desc.Component.Version, res.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch helm values template resource %q: %w", res.Name, err)
	}

	return &HelmValuesTemplate{
		ResourceName:    res.Name,
		ResourceVersion: res.Version,
		TemplateContent: string(data),
	}, nil
}

// GetHelmValuesTemplate finds the helm-values template matching chart and
// downloads its content via repo. It combines FindHelmValuesTemplate and
// FetchHelmValuesTemplate.
func GetHelmValuesTemplate(ctx context.Context, repo Repository, desc *descriptor.Descriptor, chart string) (*HelmValuesTemplate, error) {
	res, err := FindHelmValuesTemplate(desc, chart)
	if err != nil {
		return nil, fmt.Errorf("failed to find helm values template: %w", err)
	}

	return FetchHelmValuesTemplate(ctx, repo, desc, res)
}

// GetFirstHelmValuesTemplate finds the first helm-values template and downloads
// its content via repo. It combines FindFirstHelmValuesTemplate and
// FetchHelmValuesTemplate.
func GetFirstHelmValuesTemplate(ctx context.Context, repo Repository, desc *descriptor.Descriptor) (*HelmValuesTemplate, error) {
	res, err := FindFirstHelmValuesTemplate(desc)
	if err != nil {
		return nil, fmt.Errorf("failed to find first helm values template: %w", err)
	}

	return FetchHelmValuesTemplate(ctx, repo, desc, res)
}

// GetRenderingInput builds the rendering input from a descriptor. It iterates
// the component's resources and, for each resource backed by an OCI image
// access, parses its absolute reference into OCIResources keyed by resource
// name. Non-OCI resources are skipped. The descriptor's component is attached
// for template access.
//
// repoBaseURL is the repository context the CLI already holds — the base URL it
// opened the repository with, "<host>/<namespace>" — used to reconstruct FULL
// absolute references for component-local (relative) resources whose resolved
// reference is a host-less repository path. See ResourceOCIReference.
func GetRenderingInput(desc *descriptor.Descriptor, repoBaseURL string) (*RenderingInput, error) {
	ociResourceMap := make(map[string]ImageReference)

	for i := range desc.Component.Resources {
		res := desc.Component.Resources[i]

		ref, ok, err := ResourceOCIReference(res, repoBaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve OCI reference for resource %q: %w", res.Name, err)
		}
		if !ok {
			continue
		}

		parsedRef, err := ParseOCIRef(ref)
		if err != nil {
			return nil, fmt.Errorf("resource %q access contained invalid image reference: %w", res.Name, err)
		}

		ociResourceMap[res.Name] = parsedRef
	}

	return &RenderingInput{
		OCIResources: ociResourceMap,
		Component:    &desc.Component,
	}, nil
}

// Render processes a Helm values template with the provided rendering input.
// It uses Go's text/template engine with sprig functions for flexible template processing.
// The template has access to all data in the RenderingInput through dot notation.
//
// Parameters:
//   - tmpl: The HelmValuesTemplate to render
//   - input: The RenderingInput containing template data
//   - opts: Optional RenderOption functions to configure behavior (e.g., WithYAMLValidation)
//
// Returns the rendered template as a string, or an error if parsing or execution fails.
// If WithYAMLValidation is enabled, also returns an error if the output is not valid YAML.
func Render(tmpl *HelmValuesTemplate, input *RenderingInput, opts ...RenderOption) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("template is nil")
	}
	if input == nil {
		return "", fmt.Errorf("rendering input is nil")
	}

	// Apply options to config
	cfg := &renderConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Create template with custom function map
	t, err := template.New(tmpl.ResourceName).
		Option("missingkey=error").
		Funcs(getFuncMap(input.PullSecrets)).
		Parse(tmpl.TemplateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var out bytes.Buffer
	if err := t.Execute(&out, input); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	result := out.String()

	// Validate YAML if enabled
	if cfg.validateYAML {
		var jsonData any
		if err := yaml.Unmarshal([]byte(result), &jsonData); err != nil {
			return "", fmt.Errorf("rendered output is not valid YAML: %w", err)
		}
	}

	return result, nil
}

// getFuncMap creates and returns the template function map for rendering templates.
// It includes all sprig template functions (except potentially unsafe ones like env and expandenv)
// plus custom functions for JSON conversion and OCI reference parsing.
//
// Returns a template.FuncMap with all available template functions.
func getFuncMap(pullSecrets PullSecrets) template.FuncMap {
	f := sprig.TxtFuncMap()
	// Remove potentially unsafe functions
	delete(f, "env")
	delete(f, "expandenv")

	// Add custom functions
	f["toJSON"] = func(v any) string {
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}

	f["parseRef"] = ParseOCIRef

	f["pullSecretFor"] = func(ref string) string {
		return pullSecrets.Resolve(ref)
	}

	return f
}
