package helmvalues

import (
	"fmt"
	"os"
	"path/filepath"

	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// DefaultAuthClient builds an *auth.Client for talking to OCI registries.
//
// It always returns a retrying client backed by an auth cache. If a docker
// config file is available it additionally wires a credential function that
// resolves per-registry credentials from that config (including native
// credential helpers referenced by it). If no docker config is present the
// client is returned as-is for anonymous access — this is the valid
// zero-config default and is NOT an error.
//
// dockerConfigPath selects the config file:
//   - "" resolves the standard location ($DOCKER_CONFIG/config.json, else
//     ~/.docker/config.json).
//   - a non-empty value is used verbatim.
//
// If the resolved file does not exist, the anonymous client is returned.
//
// The docker-config reader is oras-go's remote/credentials store
// (credentials.NewStore + credentials.Credential). It is preferred over the
// v2 SDK's oci/credentials helpers because it produces an auth.CredentialFunc
// directly — exactly what auth.Client.Credential expects — without pulling in
// the SDK's typed-identity credential machinery, which is oriented around
// per-identity resolution rather than a whole-store credential function.
//
// TODO(ocmconfig): optional OCM config-file (~/.ocmconfig) credentials, follow-up.
func DefaultAuthClient(dockerConfigPath string) (*auth.Client, error) {
	client := &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}

	path, err := resolveDockerConfigPath(dockerConfigPath)
	if err != nil {
		return nil, err
	}

	// No config file present: anonymous access is the valid default.
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return client, nil
		}
		return nil, fmt.Errorf("failed to stat docker config %q: %w", path, err)
	}

	store, err := credentials.NewStore(path, credentials.StoreOptions{
		// Let the store fall back to the platform-default native credential
		// store when the config file itself carries no auth data.
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load docker config %q: %w", path, err)
	}

	client.Credential = credentials.Credential(store)
	return client, nil
}

// resolveDockerConfigPath returns the docker config.json path to use. An empty
// input resolves the standard location: $DOCKER_CONFIG/config.json if the env
// var is set, otherwise ~/.docker/config.json.
func resolveDockerConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home directory for docker config: %w", err)
	}
	return filepath.Join(home, ".docker", "config.json"), nil
}
