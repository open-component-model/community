package ocm

import (
	"errors"
	"log/slog"

	checksumhttpv1alpha1 "ocm.software/open-component-model/bindings/go/configuration/checksum/http/v1alpha1/spec"
	filesystemv1alpha1 "ocm.software/open-component-model/bindings/go/configuration/filesystem/v1alpha1/spec"
	gitrepository "ocm.software/open-component-model/bindings/go/git/repository"
	githubdigest "ocm.software/open-component-model/bindings/go/github/digest"
	githubresource "ocm.software/open-component-model/bindings/go/github/repository/resource"
	gpghandler "ocm.software/open-component-model/bindings/go/gpg/signing/handler"
	helmdigest "ocm.software/open-component-model/bindings/go/helm/digest"
	helmresource "ocm.software/open-component-model/bindings/go/helm/repository/resource"
	httpclient "ocm.software/open-component-model/bindings/go/http"
	httpv1alpha1 "ocm.software/open-component-model/bindings/go/http/spec/config/v1alpha1"
	"ocm.software/open-component-model/bindings/go/oci/cache"
	ocicredentials "ocm.software/open-component-model/bindings/go/oci/credentials"
	"ocm.software/open-component-model/bindings/go/oci/repository/provider"
	ociresource "ocm.software/open-component-model/bindings/go/oci/repository/resource"
	ociidentity "ocm.software/open-component-model/bindings/go/oci/spec/identity/v1"
	"ocm.software/open-component-model/bindings/go/oci/transformer"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/plugin/manager/registries/credentialtyperepository"
	"ocm.software/open-component-model/bindings/go/plugin/manager/registries/digestprocessor"
	"ocm.software/open-component-model/bindings/go/plugin/manager/registries/resource"
	"ocm.software/open-component-model/bindings/go/plugin/manager/registries/signinghandler"
	rsahandler "ocm.software/open-component-model/bindings/go/rsa/signing/handler"
	rsav1alpha1 "ocm.software/open-component-model/bindings/go/rsa/signing/v1alpha1"
	"ocm.software/open-component-model/bindings/go/runtime"
	s3repository "ocm.software/open-component-model/bindings/go/s3/repository"
	sigstorehandler "ocm.software/open-component-model/bindings/go/sigstore/signing/handler"
	wgetrepository "ocm.software/open-component-model/bindings/go/wget/repository"
)

const userAgent = "ocm-tui"

// resourcePlugin is what every remote access type registers: download, digest and credential types.
type resourcePlugin interface {
	resource.BuiltinResourceRepository
	digestprocessor.BuiltinDigestProcessorPlugin
	credentialtyperepository.BuiltinCredentialTypeSchemeProviderPlugin
}

// signingHandler is what every signing type registers: the handler and its credential types.
type signingHandler interface {
	signinghandler.BuiltinSigningHandler
	credentialtyperepository.BuiltinCredentialTypeSchemeProviderPlugin
}

// registerBuiltins mirrors the CLI's builtin.Register for everything the TUI reads or transfers.
// Input methods are left out because the TUI never constructs component versions.
func registerBuiltins(pm *manager.PluginManager, fs *filesystemv1alpha1.Config, httpCfg *httpv1alpha1.Config, checksumCfg *checksumhttpv1alpha1.Config) error {
	tempDir := *fs.TempFolder
	httpClient := httpclient.New(httpclient.WithConfig(httpCfg))

	ociResources := ociresource.NewResourceRepository(fs, ociresource.WithUserAgent(userAgent), ociresource.WithHTTPConfig(httpCfg))
	resources := []resourcePlugin{
		ociResources,
		wgetrepository.NewResourceRepository(fs, wgetrepository.WithHTTPClient(httpClient), wgetrepository.WithChecksumConfig(checksumCfg)),
		s3repository.NewResourceRepository(fs, s3repository.WithHTTPConfig(httpCfg)),
		gitrepository.NewResourceRepository(fs, gitrepository.WithHTTPClient(httpClient)),
	}

	rsaScheme := runtime.NewScheme()
	if err := rsaScheme.RegisterScheme(rsav1alpha1.Scheme); err != nil {
		return err
	}
	rsa, err := rsahandler.New(rsaScheme, true)
	if err != nil {
		return err
	}
	gpg, err := gpghandler.New(nil)
	if err != nil {
		return err
	}
	signers := []signingHandler{rsa, gpg, sigstorehandler.New(sigstorehandler.WithTempDir(tempDir))}

	errs := []error{
		pm.CredentialRepositoryRegistry.RegisterInternalCredentialRepositoryPlugin(&ocicredentials.OCICredentialRepository{}, []runtime.Type{ociidentity.Type}),
		pm.ComponentVersionRepositoryRegistry.RegisterInternalComponentVersionRepositoryPlugin(provider.NewComponentVersionRepositoryProvider(
			provider.WithTempDir(tempDir),
			provider.WithUserAgent(userAgent),
			provider.WithHTTPConfig(httpCfg),
			provider.WithBlobCacheOptions(&cache.Options{RemotePolicy: cache.RemotePolicyIfNotPresent}),
			provider.WithReferenceCacheOptions(&cache.Options{RemotePolicy: cache.RemotePolicyIfNotPresent}),
		)),
		pm.BlobTransformerRegistry.RegisterInternalBlobTransformerPlugin(transformer.New(slog.Default())),
		pm.ResourcePluginRegistry.RegisterInternalResourcePlugin(helmresource.NewResourceRepository(fs, helmresource.WithHTTPConfig(httpCfg))),
		pm.DigestProcessorRegistry.RegisterInternalDigestProcessorPlugin(helmdigest.NewDigestProcessor(tempDir)),
	}
	for _, r := range resources {
		errs = append(errs,
			pm.ResourcePluginRegistry.RegisterInternalResourcePlugin(r),
			pm.DigestProcessorRegistry.RegisterInternalDigestProcessorPlugin(r),
			pm.CredentialTypeRegistry.RegisterInternalCredentialTypeSchemeProvider(r),
		)
	}
	// GitHub digests are computed by a separate processor, not by the resource repository.
	githubResources := githubresource.NewResourceRepository(githubresource.WithHTTPClient(httpClient))
	githubDigest := githubdigest.NewDigestProcessor(githubresource.WithHTTPClient(httpClient))
	errs = append(errs,
		pm.ResourcePluginRegistry.RegisterInternalResourcePlugin(githubResources),
		pm.CredentialTypeRegistry.RegisterInternalCredentialTypeSchemeProvider(githubResources),
		pm.DigestProcessorRegistry.RegisterInternalDigestProcessorPlugin(githubDigest),
		pm.CredentialTypeRegistry.RegisterInternalCredentialTypeSchemeProvider(githubDigest),
	)
	for _, s := range signers {
		errs = append(errs,
			pm.SigningRegistry.RegisterInternalComponentSignatureHandler(s),
			pm.CredentialTypeRegistry.RegisterInternalCredentialTypeSchemeProvider(s),
		)
	}
	return errors.Join(errs...)
}
