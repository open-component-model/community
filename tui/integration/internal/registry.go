// Package internal provides the test registry and fixtures for the TUI integration tests.
// It follows bindings/go/cli/integration/internal in the OCM repository.
package internal

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/modules/registry"
	"golang.org/x/crypto/bcrypt"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	"ocm.software/open-component-model/bindings/go/blob/direct"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci"
	urlresolver "ocm.software/open-component-model/bindings/go/oci/resolver/url"
)

const distributionRegistryImage = "registry:3.0.0"

// Registry is a password-protected OCI registry running in a container.
type Registry struct {
	Address, Host, Port, User, Password string
}

// StartRegistry starts a registry container that is terminated when the test ends.
func StartRegistry(t *testing.T) *Registry {
	t.Helper()
	r := require.New(t)

	user, password := "ocm", fmt.Sprintf("tui-%d", time.Now().UnixNano())
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	r.NoError(err)
	htpasswd := filepath.Join(t.TempDir(), "htpasswd")
	r.NoError(os.WriteFile(htpasswd, []byte(fmt.Sprintf("%s:%s", user, hash)), 0o600))

	container, err := registry.Run(t.Context(), distributionRegistryImage,
		registry.WithHtpasswdFile(htpasswd),
		testcontainers.WithEnv(map[string]string{"REGISTRY_VALIDATION_DISABLED": "true"}),
		testcontainers.WithLogger(log.TestLogger(t)),
		testcontainers.WithName(fmt.Sprintf("%s-%d", regexp.MustCompile(`[^a-z0-9_.-]`).ReplaceAllString(strings.ToLower(t.Name()), "-"), time.Now().UnixNano())),
	)
	r.NoError(err)
	t.Cleanup(func() { r.NoError(testcontainers.TerminateContainer(container)) })

	address, err := container.HostAddress(t.Context())
	r.NoError(err)
	host, port, err := net.SplitHostPort(address)
	r.NoError(err)
	return &Registry{Address: address, Host: host, Port: port, User: user, Password: password}
}

// Reference returns the TUI reference "http://<registry>//<component>:<version>".
func (reg *Registry) Reference(component, version string) string {
	return fmt.Sprintf("http://%s//%s:%s", reg.Address, component, version)
}

// UseConfig writes an OCM config with the registry credentials and points the TUI at it.
// HOME is moved to a temporary directory so the developer's own config and plugins are not loaded.
func (reg *Registry) UseConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, "ocmconfig.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte(fmt.Sprintf(`
type: generic.config.ocm.software/v1
configurations:
- type: credentials.config.ocm.software
  consumers:
  - identity:
      type: OCIRegistry
      hostname: %q
      port: %q
      scheme: http
    credentials:
    - type: Credentials/v1
      properties:
        username: %q
        password: %q
`, reg.Host, reg.Port, reg.User, reg.Password)), 0o600))
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OCM_CONFIG", cfg)
}

// PushComponentVersion uploads a component version with one local blob resource holding data.
func (reg *Registry) PushComponentVersion(t *testing.T, component, version, resource string, data []byte) {
	t.Helper()
	r := require.New(t)

	resolver, err := urlresolver.New(
		urlresolver.WithBaseURL(reg.Address),
		urlresolver.WithPlainHTTP(true),
		urlresolver.WithBaseClient(&auth.Client{
			Client:     retry.DefaultClient,
			Header:     http.Header{"User-Agent": []string{"ocm.software/tui-integration-test"}},
			Credential: auth.StaticCredential(reg.Address, auth.Credential{Username: reg.User, Password: reg.Password}),
		}),
	)
	r.NoError(err)
	repo, err := oci.NewRepository(oci.WithResolver(resolver), oci.WithTempDir(t.TempDir()))
	r.NoError(err)

	res := &descriptor.Resource{
		ElementMeta: descriptor.ElementMeta{ObjectMeta: descriptor.ObjectMeta{Name: resource, Version: version}},
		Type:        "plainText",
		Relation:    descriptor.LocalRelation,
		Access:      &v2.LocalBlob{},
	}
	res, err = repo.AddLocalResource(t.Context(), component, version, res, direct.NewFromBytes(data))
	r.NoError(err)

	desc := &descriptor.Descriptor{Meta: descriptor.Meta{Version: "v2"}}
	desc.Component.Name, desc.Component.Version = component, version
	desc.Component.Provider.Name = "ocm.software"
	desc.Component.Resources = []descriptor.Resource{*res}
	r.NoError(repo.AddComponentVersion(t.Context(), desc))
}
