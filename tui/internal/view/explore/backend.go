package explore

import (
	"context"
	"fmt"
	"path/filepath"

	"ocm.software/open-component-model/bindings/go/blob"
	"ocm.software/open-component-model/bindings/go/blob/filesystem"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci/compref"
	"ocm.software/open-component-model/bindings/go/repository"

	"ext.ocm.software/tui/internal/ocm"
)

// Connect returns an OpenFunc that resolves references through the OCM runtime.
func Connect(rt *ocm.Runtime) OpenFunc {
	return func(ctx context.Context, reference string) (Repository, string, string, error) {
		ref, err := compref.Parse(reference, compref.IgnoreSemverCompatibility())
		if err != nil {
			return nil, "", "", fmt.Errorf("parsing reference %q: %w", reference, err)
		}
		repo, err := rt.Repository(ctx, ref.Repository)
		if err != nil {
			return nil, "", "", err
		}
		return &ocmRepository{ComponentVersionRepository: repo, rt: rt}, ref.Component, ref.Version, nil
	}
}

type ocmRepository struct {
	repository.ComponentVersionRepository
	rt *ocm.Runtime
}

// DownloadResource writes a resource into dir and returns the absolute path written.
func (r *ocmRepository) DownloadResource(ctx context.Context, component, version string, res *descriptor.Resource, dir string) (string, error) {
	identity := res.ToIdentity()
	var local v2.LocalBlob
	var data blob.ReadOnlyBlob
	var err error
	if access := res.GetAccess(); access != nil && v2.Scheme.Convert(access, &local) == nil {
		data, _, err = r.GetLocalResource(ctx, component, version, identity)
	} else {
		data, err = r.downloadRemote(ctx, res)
	}
	if err != nil {
		return "", fmt.Errorf("downloading resource %s: %w", res.Name, err)
	}
	path := filepath.Join(dir, identity.String())
	if err := filesystem.CopyBlobToOSPath(data, path); err != nil {
		return "", fmt.Errorf("writing resource %s: %w", res.Name, err)
	}
	return filepath.Abs(path)
}

func (r *ocmRepository) downloadRemote(ctx context.Context, res *descriptor.Resource) (blob.ReadOnlyBlob, error) {
	plugin, err := r.rt.Plugins.ResourcePluginRegistry.GetResourcePlugin(ctx, res.GetAccess())
	if err != nil {
		return nil, fmt.Errorf("finding resource plugin: %w", err)
	}
	identity, err := plugin.GetResourceCredentialConsumerIdentity(ctx, res)
	if err != nil {
		return plugin.DownloadResource(ctx, res, nil)
	}
	creds, err := r.rt.Resolve(ctx, identity)
	if err != nil {
		return nil, err
	}
	return plugin.DownloadResource(ctx, res, creds)
}
