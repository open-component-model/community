package helmvalues

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewAuthClient_Anonymous verifies that when neither an OCM config nor a
// docker config exists, the client still builds and resolves to empty
// (anonymous) credentials for an arbitrary host.
func TestNewAuthClient_Anonymous(t *testing.T) {
	// Point both config sources at empty temp dirs so nothing on the host
	// (real ~/.ocmconfig or ~/.docker) is consulted.
	t.Setenv("OCM_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	c, err := NewAuthClient(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Cache)
	require.NotNil(t, c.Credential)

	cred, err := c.Credential(context.Background(), "registry.example.com")
	require.NoError(t, err)
	require.Empty(t, cred.Username)
	require.Empty(t, cred.Password)
}

// TestNewAuthClient_MalformedConfigFailsLoudly verifies that a malformed OCM
// config is surfaced as an error rather than silently swallowed.
func TestNewAuthClient_MalformedConfigFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ocmconfig.yaml")
	require.NoError(t, os.WriteFile(path, []byte("this: [is: not: valid: yaml"), 0o600))
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	_, err := NewAuthClient(context.Background(), path)
	require.Error(t, err)
}

// TestNewAuthClient_ResolvesDockerCredential verifies that credentials from an
// implicit docker config.json (via $DOCKER_CONFIG) resolve through the graph's
// default DockerConfig repository, even without any OCM config present.
func TestNewAuthClient_ResolvesDockerCredential(t *testing.T) {
	// registry.example.com -> user:pass ("dXNlcjpwYXNz").
	dockerDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dockerDir, "config.json"), []byte(`{
  "auths": {
    "registry.example.com": {"auth": "dXNlcjpwYXNz"}
  }
}`), 0o600))
	t.Setenv("DOCKER_CONFIG", dockerDir)
	// No OCM config file.
	t.Setenv("OCM_CONFIG", filepath.Join(t.TempDir(), "does-not-exist.yaml"))

	c, err := NewAuthClient(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, c.Credential)

	cred, err := c.Credential(context.Background(), "registry.example.com")
	require.NoError(t, err)
	require.Equal(t, "user", cred.Username)
	require.Equal(t, "pass", cred.Password)

	// An unknown host resolves to anonymous rather than erroring.
	other, err := c.Credential(context.Background(), "unknown.example.com")
	require.NoError(t, err)
	require.Empty(t, other.Username)
}

// TestNewAuthClient_ResolvesOCMConfigConsumer verifies that a Credentials/v1
// consumer for an OCIRegistry hostname declared in ~/.ocmconfig resolves
// through the credential graph.
func TestNewAuthClient_ResolvesOCMConfigConsumer(t *testing.T) {
	// Keep docker config empty so the only credential source is the OCM config.
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	ocmDir := t.TempDir()
	ocmConfig := filepath.Join(ocmDir, ".ocmconfig")
	const content = `type: generic.config.ocm.software/v1
configurations:
  - type: credentials.config.ocm.software
    consumers:
      - identities:
          - type: OCIRegistry
            hostname: ocm.example.com
        credentials:
          - type: OCICredentials/v1
            username: ocm-user
            password: ocm-pass
`
	require.NoError(t, os.WriteFile(ocmConfig, []byte(content), 0o600))
	t.Setenv("OCM_CONFIG", ocmConfig)

	c, err := NewAuthClient(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, c.Credential)

	cred, err := c.Credential(context.Background(), "ocm.example.com")
	require.NoError(t, err)
	require.Equal(t, "ocm-user", cred.Username)
	require.Equal(t, "ocm-pass", cred.Password)

	// A host without configured credentials falls back to anonymous.
	other, err := c.Credential(context.Background(), "unconfigured.example.com")
	require.NoError(t, err)
	require.Empty(t, other.Username)
}

// TestNewAuthClient_ExplicitPath verifies an explicit ocmConfigPath is honored
// over environment/default discovery.
func TestNewAuthClient_ExplicitPath(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	// Point env at a bad file to prove the explicit arg takes precedence.
	t.Setenv("OCM_CONFIG", filepath.Join(t.TempDir(), "ignored.yaml"))

	ocmConfig := filepath.Join(t.TempDir(), "explicit.yaml")
	const content = `type: generic.config.ocm.software/v1
configurations:
  - type: credentials.config.ocm.software
    consumers:
      - identities:
          - type: OCIRegistry
            hostname: explicit.example.com
        credentials:
          - type: OCICredentials/v1
            username: exp-user
            password: exp-pass
`
	require.NoError(t, os.WriteFile(ocmConfig, []byte(content), 0o600))

	c, err := NewAuthClient(context.Background(), ocmConfig)
	require.NoError(t, err)

	cred, err := c.Credential(context.Background(), "explicit.example.com")
	require.NoError(t, err)
	require.Equal(t, "exp-user", cred.Username)
	require.Equal(t, "exp-pass", cred.Password)
}
