package ocm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"ocm.software/open-component-model/bindings/go/blob"
	"ocm.software/open-component-model/bindings/go/repository/component/resolvers"
	"ocm.software/open-component-model/bindings/go/blob/filesystem"
	"ocm.software/open-component-model/bindings/go/credentials"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci/compref"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/repository"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transfer"
	graphRuntime "ocm.software/open-component-model/bindings/go/transform/graph/runtime"
	transformv1alpha1 "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"

	"ext.ocm.software/tui/fetch"
)

// NewFetcherFactory creates a FetcherFactory from the runtime.
func NewFetcherFactory(rt *Runtime) fetch.FetcherFactory {
	return func(ctx context.Context, reference string) (fetch.ComponentFetcher, string, string, error) {
		ref, err := compref.Parse(reference, compref.IgnoreSemverCompatibility())
		if err != nil {
			return nil, "", "", fmt.Errorf("invalid reference %q: %w", reference, err)
		}

		repo, err := getRepo(ctx, rt, ref)
		if err != nil {
			return nil, "", "", err
		}

		return &componentFetcher{repo: repo}, ref.Component, ref.Version, nil
	}
}

type componentFetcher struct {
	repo repository.ComponentVersionRepository
}

func (f *componentFetcher) ListVersions(ctx context.Context, component string) ([]string, error) {
	return f.repo.ListComponentVersions(ctx, component)
}

func (f *componentFetcher) GetDescriptor(ctx context.Context, component, version string) (*descriptor.Descriptor, error) {
	return f.repo.GetComponentVersion(ctx, component, version)
}

// NewResourceDownloader creates a ResourceDownloader from the runtime.
func NewResourceDownloader(rt *Runtime) fetch.ResourceDownloader {
	return &resourceDownloader{rt: rt}
}

type resourceDownloader struct {
	rt *Runtime
}

func (d *resourceDownloader) DownloadResource(ctx context.Context, reference, component, version string, res *descriptor.Resource, outputDir string) (string, error) {
	ref, err := compref.Parse(reference, compref.IgnoreSemverCompatibility())
	if err != nil {
		return "", fmt.Errorf("parsing reference: %w", err)
	}
	ref.Component = component
	ref.Version = version

	repo, err := getRepo(ctx, d.rt, ref)
	if err != nil {
		return "", err
	}

	identity := res.ToIdentity()
	data, err := downloadResourceData(ctx, d.rt.PluginManager, d.rt.CredentialGraph, component, version, repo, res, identity)
	if err != nil {
		return "", err
	}

	outputPath := fmt.Sprintf("%s/%s", outputDir, identity.String())
	outputDir2 := outputPath
	_ = outputDir2
	if err := filesystem.CopyBlobToOSPath(data, outputPath); err != nil {
		return "", fmt.Errorf("saving resource: %w", err)
	}
	return outputPath, nil
}

// downloadResourceData handles both local blob and remote plugin resources.
func downloadResourceData(ctx context.Context, pm *manager.PluginManager, credGraph credentials.Resolver,
	component, version string, repo repository.ComponentVersionRepository,
	res *descriptor.Resource, identity runtime.Identity,
) (blob.ReadOnlyBlob, error) {
	access := res.GetAccess()

	// Check if local blob access.
	if isLocal(access) {
		data, _, err := repo.GetLocalResource(ctx, component, version, identity)
		return data, err
	}

	// Remote: use resource plugin.
	plugin, err := pm.ResourcePluginRegistry.GetResourcePlugin(ctx, access)
	if err != nil {
		return nil, fmt.Errorf("getting resource plugin for %q: %w", access.GetType(), err)
	}

	var creds map[string]string
	if credIdentity, err := plugin.GetResourceCredentialConsumerIdentity(ctx, res); err == nil {
		if creds, err = credGraph.Resolve(ctx, credIdentity); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return nil, fmt.Errorf("resolving credentials: %w", err)
		}
	}

	return plugin.DownloadResource(ctx, res, creds)
}

func isLocal(access runtime.Typed) bool {
	if access == nil {
		return false
	}
	var local v2.LocalBlob
	return v2.Scheme.Convert(access, &local) == nil
}

// NewTransferExecutor creates a TransferExecutor from the runtime.
func NewTransferExecutor(rt *Runtime) fetch.TransferExecutor {
	return &transferExecutor{rt: rt}
}

type transferExecutor struct {
	rt *Runtime
}

func (t *transferExecutor) BuildGraph(ctx context.Context, source, target string, opts fetch.TransferOptions) (*transformv1alpha1.TransformationGraphDefinition, error) {
	fromSpec, err := compref.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("invalid source: %w", err)
	}

	sourceRepo, err := getRepo(ctx, t.rt, fromSpec)
	if err != nil {
		return nil, err
	}

	toSpec, err := compref.ParseRepository(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}

	copyMode := transfer.CopyModeLocalBlobResources
	if opts.CopyResources {
		copyMode = transfer.CopyModeAllResources
	}
	uploadType := transfer.UploadAsDefault
	switch opts.UploadAs {
	case "localBlob":
		uploadType = transfer.UploadAsLocalBlob
	case "ociArtifact":
		uploadType = transfer.UploadAsOciArtifact
	}

	resolver := &repoResolver{rt: t.rt, repo: sourceRepo, spec: fromSpec.Repository}
	return transfer.BuildGraphDefinition(ctx,
		transfer.WithTransfer(
			transfer.Component(fromSpec.Component, fromSpec.Version),
			transfer.ToRepositorySpec(toSpec),
			transfer.FromResolver(resolver),
		),
		transfer.WithRecursive(opts.Recursive),
		transfer.WithCopyMode(copyMode),
		transfer.WithUploadType(uploadType),
	)
}

func (t *transferExecutor) Execute(ctx context.Context, tgd *transformv1alpha1.TransformationGraphDefinition, progressCh chan<- fetch.TransferProgress) error {
	b := transfer.NewDefaultBuilder(
		t.rt.PluginManager.ComponentVersionRepositoryRegistry,
		t.rt.PluginManager.ResourcePluginRegistry,
		t.rt.CredentialGraph,
	)

	eventCh := make(chan graphRuntime.ProgressEvent, 16)
	graph, err := b.WithEvents(eventCh).BuildAndCheck(tgd)
	if err != nil {
		close(progressCh)
		return fmt.Errorf("building transformation graph: %w", err)
	}

	nodeCount := graph.NodeCount()
	fwd := &slogForwarder{fallback: slog.Default().Handler(), ch: progressCh}
	slog.SetDefault(slog.New(fwd))

	eventsDone := make(chan struct{})
	go func() {
		defer close(eventsDone)
		completed := 0
		for event := range graph.Events() {
			if event.State == graphRuntime.Completed || event.State == graphRuntime.Failed {
				completed++
			}
			name := event.State.String()
			if event.Transformation != nil {
				name = fmt.Sprintf("%s [%s]: %s", event.Transformation.ID, event.Transformation.Type.Name, event.State.String())
			}
			progressCh <- fetch.TransferProgress{Step: name, Total: nodeCount, Current: completed}
		}
	}()

	processErr := graph.Process(ctx)
	<-eventsDone
	slog.SetDefault(slog.New(fwd.fallback))
	fwd.stop()
	close(progressCh)

	if processErr != nil {
		return fmt.Errorf("transfer failed: %w", processErr)
	}
	return nil
}

// --- Resolver ---

// repoResolver implements resolvers.ComponentVersionRepositoryResolver
// by reusing a single repository connection.
type repoResolver struct {
	rt   *Runtime
	repo repository.ComponentVersionRepository
	spec runtime.Typed
}

var _ resolvers.ComponentVersionRepositoryResolver = (*repoResolver)(nil)

func (r *repoResolver) GetComponentVersionRepositoryForComponent(ctx context.Context, component, version string) (repository.ComponentVersionRepository, error) {
	return r.repo, nil
}

func (r *repoResolver) GetComponentVersionRepositoryForSpecification(ctx context.Context, specification runtime.Typed) (repository.ComponentVersionRepository, error) {
	return getRepo(ctx, r.rt, &compref.Ref{Repository: specification})
}

func (r *repoResolver) GetRepositorySpecificationForComponent(ctx context.Context, component, version string) (runtime.Typed, error) {
	return r.spec, nil
}

// --- Helpers ---

func getRepo(ctx context.Context, rt *Runtime, ref *compref.Ref) (repository.ComponentVersionRepository, error) {
	repoSpec := ref.Repository
	if repoSpec == nil {
		return nil, fmt.Errorf("no repository in reference")
	}

	// Resolve credentials for the repository.
	var creds map[string]string
	identity, err := rt.PluginManager.ComponentVersionRepositoryRegistry.GetComponentVersionRepositoryCredentialConsumerIdentity(ctx, repoSpec)
	if err == nil && rt.CredentialGraph != nil {
		if creds, err = rt.CredentialGraph.Resolve(ctx, identity); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return nil, fmt.Errorf("resolving repository credentials: %w", err)
		}
	}

	repo, err := rt.PluginManager.ComponentVersionRepositoryRegistry.GetComponentVersionRepository(ctx, repoSpec, creds)
	if err != nil {
		return nil, fmt.Errorf("connecting to repository: %w", err)
	}

	return repo, nil
}

// --- slog forwarder ---

type slogForwarder struct {
	fallback slog.Handler
	ch       chan<- fetch.TransferProgress
	stopped  bool
}

func (h *slogForwarder) stop()                                        { h.stopped = true }
func (h *slogForwarder) Enabled(_ context.Context, _ slog.Level) bool { return !h.stopped }
func (h *slogForwarder) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogForwarder{fallback: h.fallback.WithAttrs(attrs), ch: h.ch, stopped: h.stopped}
}
func (h *slogForwarder) WithGroup(name string) slog.Handler {
	return &slogForwarder{fallback: h.fallback.WithGroup(name), ch: h.ch, stopped: h.stopped}
}
func (h *slogForwarder) Handle(_ context.Context, r slog.Record) error {
	if h.stopped {
		return nil
	}
	msg := r.Message
	r.Attrs(func(a slog.Attr) bool { msg += fmt.Sprintf(" %s=%v", a.Key, a.Value); return true })
	select {
	case h.ch <- fetch.TransferProgress{Step: msg, IsLog: true}:
	default:
	}
	return nil
}
