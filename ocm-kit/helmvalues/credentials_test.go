package helmvalues

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultAuthClient_Anonymous(t *testing.T) {
	// Point DOCKER_CONFIG at an empty dir so no real ~/.docker is consulted.
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	c, err := DefaultAuthClient("")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Cache)
}

func TestDefaultAuthClient_MissingExplicitConfigIsAnonymous(t *testing.T) {
	// An explicit path to a non-existent file must yield the anonymous client
	// (no error): anonymous is the valid zero-config default.
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	c, err := DefaultAuthClient(missing)
	require.NoError(t, err)
	require.NotNil(t, c)
	// No credential function is wired for a missing config.
	require.Nil(t, c.Credential)
}

func TestDefaultAuthClient_ResolvesDockerCredential(t *testing.T) {
	// Hermetic: a temp docker config.json with a base64 basic-auth entry.
	// registry.example.com -> user:pass ("dXNlcjpwYXNz").
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	const content = `{
  "auths": {
    "registry.example.com": {
      "auth": "dXNlcjpwYXNz"
    }
  }
}`
	require.NoError(t, os.WriteFile(cfg, []byte(content), 0o600))

	c, err := DefaultAuthClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Credential, "a credential function must be wired when a config exists")

	cred, err := c.Credential(context.Background(), "registry.example.com")
	require.NoError(t, err)
	require.Equal(t, "user", cred.Username)
	require.Equal(t, "pass", cred.Password)
}

func TestDefaultAuthClient_DefaultPathViaDockerConfigEnv(t *testing.T) {
	// With dockerConfigPath == "" the helper must honor $DOCKER_CONFIG.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{
  "auths": {
    "registry.example.com": {"auth": "dXNlcjpwYXNz"}
  }
}`), 0o600))
	t.Setenv("DOCKER_CONFIG", dir)

	c, err := DefaultAuthClient("")
	require.NoError(t, err)
	require.NotNil(t, c.Credential)

	cred, err := c.Credential(context.Background(), "registry.example.com")
	require.NoError(t, err)
	require.Equal(t, "user", cred.Username)
	require.Equal(t, "pass", cred.Password)
}
