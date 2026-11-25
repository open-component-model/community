package compver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	content := []byte("replicas: 1")
	dir := writeTestCTF(t, map[ComponentVersion][]byte{{Name: "acme.org/app", Version: "1.0.0"}: content})
	ref := "ctf::" + dir + "//acme.org/app:1.0.0"
	archive := tarGzDir(t, dir)
	for _, tc := range []struct{ input, ref string }{
		{dir, ref},
		{ref, ref},
		{archive, "ctf::" + archive + "//acme.org/app:1.0.0"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			opened, err := Open(t.Context(), tc.input)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, opened.Close()) })
			require.Equal(t, CTFPlaceholderHost, opened.BaseURL)
			require.Equal(t, "acme.org/app", opened.Descriptor.Component.Name)
			require.Equal(t, "1.0.0", opened.Descriptor.Component.Version)
			require.Equal(t, tc.ref, opened.Ref)
			data, err := opened.Repository.ResourceBytes(t.Context(), "acme.org/app", "1.0.0", testResourceName)
			require.NoError(t, err)
			require.Equal(t, content, data)
		})
	}
}

func TestOpenReferenceErrors(t *testing.T) {
	for _, tc := range []struct{ ref, message string }{
		{"https://registry.example.org/components//example.org/app", "must include a version"},
		{"https://registry.example.org/components//example.org/app:1.0.0@sha256:" + strings.Repeat("a", 64), "digest-pinned"},
		{"https://registry.example.org/components//invalid:1.0.0", "invalid component name"},
	} {
		t.Run(tc.message, func(t *testing.T) {
			_, err := Open(t.Context(), tc.ref)
			require.ErrorContains(t, err, tc.message)
		})
	}
}
