package transfer

import (
	"context"
	"fmt"

	"ocm.software/open-component-model/bindings/go/oci/compref"
	ctfv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/ctf"
	"ocm.software/open-component-model/bindings/go/transfer"
	transferv1alpha1 "ocm.software/open-component-model/bindings/go/transfer/v1alpha1/spec"
	graphruntime "ocm.software/open-component-model/bindings/go/transform/graph/runtime"
	transformv1alpha1 "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"

	"ext.ocm.software/tui/internal/ocm"
)

// Options are the user-selectable transfer settings.
type Options struct {
	Recursive     bool
	CopyResources bool
	UploadAs      transferv1alpha1.UploadType
}

// Backend returns a Transferer that runs transfers through the OCM runtime.
func Backend(rt *ocm.Runtime) Transferer { return &backend{rt} }

type backend struct{ rt *ocm.Runtime }

// BuildTransfer builds the transformation graph that transfers source
// ("repo//component:version") into the target repository.
func (b *backend) BuildTransfer(ctx context.Context, source, target string, opts Options) (*transformv1alpha1.TransformationGraphDefinition, error) {
	from, err := compref.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parsing source %q: %w", source, err)
	}
	to, err := compref.ParseRepository(target, compref.WithCTFAccessMode(ctfv1.AccessModeReadWrite+"|"+ctfv1.AccessModeCreate))
	if err != nil {
		return nil, fmt.Errorf("parsing target %q: %w", target, err)
	}
	repo, err := b.rt.Repository(ctx, from.Repository)
	if err != nil {
		return nil, err
	}
	cfg := &transferv1alpha1.Config{CopyMode: transferv1alpha1.CopyModeLocalBlobResources, UploadType: opts.UploadAs}
	if opts.Recursive {
		cfg.Recursive = transferv1alpha1.RecursiveInfinite
	}
	if opts.CopyResources {
		cfg.CopyMode = transferv1alpha1.CopyModeAllResources
	}
	return transfer.BuildGraphDefinition(ctx, cfg, nil, transfer.Mapping{
		Components: []transfer.ComponentID{{Component: from.Component, Version: from.Version}},
		Target:     to,
		Resolver:   transfer.NewRepositoryResolver(repo, from.Repository),
	})
}

// Transfer executes the graph and reports one line per node state change.
// progress is closed when the transfer finishes.
func (b *backend) Transfer(ctx context.Context, tgd *transformv1alpha1.TransformationGraphDefinition, progress chan<- Progress) error {
	defer close(progress)
	graph, err := transfer.NewDefaultBuilder(
		b.rt.Plugins.ComponentVersionRepositoryRegistry,
		b.rt.Plugins.ResourcePluginRegistry,
		b.rt.Credentials,
		transfer.WithHTTPConfig(b.rt.HTTP),
	).WithEvents(make(chan graphruntime.ProgressEvent, 16)).BuildAndCheck(tgd)
	if err != nil {
		return fmt.Errorf("building transformation graph: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		total, finished := graph.NodeCount(), 0
		for e := range graph.Events() {
			if e.State != graphruntime.Running {
				finished++
			}
			line := fmt.Sprintf("[%d/%d] %s [%s]: %s", finished, total, e.Transformation.ID, e.Transformation.Type.Name, e.State)
			if e.Err != nil {
				line += ": " + e.Err.Error()
			}
			progress <- Progress{Line: line, Finished: finished, Total: total}
		}
	}()
	err = graph.Process(ctx)
	<-done
	return err
}
