package helmvalues

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOCIRef(t *testing.T) {
	ref, err := ParseOCIRef("ghcr.io/acme/app:v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", ref.Host)
	require.Equal(t, "acme/app", ref.Repository)
	require.Equal(t, "v1.2.3", ref.Tag)
	require.Empty(t, ref.Digest)

	require.Equal(t, "ghcr.io/acme/app:v1.2.3", ref.String())
}

func TestParseOCIRef_Digest(t *testing.T) {
	const dig = "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"
	d, err := ParseOCIRef("ghcr.io/acme/app@" + dig)
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", d.Host)
	require.Equal(t, "acme/app", d.Repository)
	require.Equal(t, dig, d.Digest)
}
