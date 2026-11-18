// Package compver provides shared access to OCM component versions.
package compver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/oci/compref"
	ctfv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/ctf"
	ociv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/oci"
)

// Component is a loaded component version. Close it when finished to remove
// temporary repository files.
type Component struct {
	Repository *OCIRepository
	Descriptor *descriptor.Descriptor
	BaseURL    string
	Ref        string
}

// Close removes the temporary files owned by the component's repository.
func (c *Component) Close() error { return c.Repository.Close() }

const ctfPrefix = "ctf::"

// Open resolves an OCI or CTF reference and loads its component descriptor.
// A bare CTF path selects its only component version. Registry credentials are
// discovered from the OCM and Docker configuration.
func Open(ctx context.Context, arg string) (_ *Component, err error) {
	ref, err := compref.Parse(arg)
	if err != nil {
		if strings.Contains(arg, "//") {
			return nil, err
		}
		// Bare CTF paths select their single component version below.
		ref = &compref.Ref{Repository: &ctfv1.Repository{FilePath: strings.TrimPrefix(arg, ctfPrefix)}}
	}
	if ref.Digest != "" {
		return nil, errors.New("digest-pinned component references are not supported")
	}
	if ref.Component != "" && ref.Version == "" {
		return nil, errors.New("component reference must include a version")
	}
	component := &Component{Ref: arg, BaseURL: CTFPlaceholderHost}
	defer func() {
		if err != nil && component.Repository != nil {
			_ = component.Repository.Close()
		}
	}()
	switch spec := ref.Repository.(type) {
	case *ociv1.Repository:
		repoURL := spec.BaseUrl
		if spec.SubPath != "" {
			repoURL += "/" + spec.SubPath
		}
		client, err := NewAuthClient(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("create auth client: %w", err)
		}
		component.Repository, err = OpenRepository(repoURL, WithAuthClient(client))
		if err != nil {
			return nil, err
		}
		component.BaseURL = repoURL
		if _, hostPath, found := strings.Cut(repoURL, "://"); found {
			component.BaseURL = hostPath
		}
	case *ctfv1.Repository:
		repo, err := OpenCTF(ctx, spec.FilePath)
		if err != nil {
			return nil, err
		}
		component.Repository = repo.OCIRepository
		if ref.Component == "" {
			cvs, err := repo.ComponentVersions(ctx)
			if err != nil {
				return nil, err
			}
			switch len(cvs) {
			case 0:
				return nil, fmt.Errorf("no component versions in CTF %q", spec.FilePath)
			case 1:
				ref.Component, ref.Version = cvs[0].Name, cvs[0].Version
			default:
				lines := make([]string, len(cvs))
				for i, c := range cvs {
					lines[i] = c.Name + ":" + c.Version
				}
				return nil, fmt.Errorf("CTF %q holds several component versions, pass %s%s//<name>:<version> with one of:\n%s",
					spec.FilePath, ctfPrefix, spec.FilePath, strings.Join(lines, "\n"))
			}
		}
		component.Ref = ctfPrefix + spec.FilePath + "//" + ref.Component + ":" + ref.Version
	default:
		return nil, fmt.Errorf("unsupported repository type %T", ref.Repository)
	}
	component.Descriptor, err = component.Repository.GetComponentVersion(ctx, ref.Component, ref.Version)
	if err != nil {
		return nil, fmt.Errorf("get component version %s:%s: %w", ref.Component, ref.Version, err)
	}
	return component, nil
}
