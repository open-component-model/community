package helmvalues

import (
	"context"
	"fmt"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

// FakeRepository is an in-memory Repository for tests.
type FakeRepository struct {
	Descriptor *descriptor.Descriptor
	Blobs      map[string][]byte // resource name -> content
}

func (f *FakeRepository) GetComponentVersion(_ context.Context, _, _ string) (*descriptor.Descriptor, error) {
	if f.Descriptor == nil {
		return nil, fmt.Errorf("no descriptor configured")
	}
	return f.Descriptor, nil
}

func (f *FakeRepository) ResourceBytes(_ context.Context, _, _, resourceName string) ([]byte, error) {
	b, ok := f.Blobs[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource %q: %w", resourceName, ErrNotFound)
	}
	return b, nil
}
