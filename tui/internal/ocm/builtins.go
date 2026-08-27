package ocm

import (
	"errors"
	"log/slog"

	ocicredentials "ocm.software/open-component-model/bindings/go/oci/credentials"
	"ocm.software/open-component-model/bindings/go/oci/repository/provider"
	ocires "ocm.software/open-component-model/bindings/go/oci/repository/resource"
	credidentityv1 "ocm.software/open-component-model/bindings/go/oci/spec/credentials/identity/v1"
	"ocm.software/open-component-model/bindings/go/oci/transformer"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"

	// Import the credential spec packages for their init() side effects.
	// This registers DockerConfig/v1 in the credential repository scheme.
	_ "ocm.software/open-component-model/bindings/go/oci/spec/credentials"
	_ "ocm.software/open-component-model/bindings/go/oci/spec/credentials/v1"
)

const userAgent = "ocm-tui"

// registerBuiltins mirrors what the CLI's builtin.Register does,
// registering the OCI plugins needed for browsing, downloading,
// and credential resolution.
func registerBuiltins(pm *manager.PluginManager) error {
	// 1. OCI credential repository (same as cli/internal/plugin/builtin/credentials/oci)
	if err := pm.CredentialRepositoryRegistry.RegisterInternalCredentialRepositoryPlugin(
		&ocicredentials.OCICredentialRepository{},
		[]runtime.Type{credidentityv1.Type},
	); err != nil {
		return err
	}

	// 2. OCI component version repository + resource plugin (same as cli/internal/plugin/builtin/oci)
	cvRepoProvider := provider.NewComponentVersionRepositoryProvider(
		provider.WithUserAgent(userAgent),
	)
	resPlugin := ocires.NewResourceRepository(nil, ocires.WithUserAgent(userAgent))
	blobTransformer := transformer.New(slog.Default())

	return errors.Join(
		pm.ComponentVersionRepositoryRegistry.RegisterInternalComponentVersionRepositoryPlugin(cvRepoProvider),
		pm.ResourcePluginRegistry.RegisterInternalResourcePlugin(resPlugin),
		pm.DigestProcessorRegistry.RegisterInternalDigestProcessorPlugin(resPlugin),
		pm.BlobTransformerRegistry.RegisterInternalBlobTransformerPlugin(blobTransformer),
	)
}
