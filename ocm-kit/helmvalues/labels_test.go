package helmvalues

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

func resWithLabel(name string, labels ...descriptor.Label) descriptor.Resource {
	return descriptor.Resource{
		ElementMeta: descriptor.ElementMeta{
			ObjectMeta: descriptor.ObjectMeta{Name: name, Labels: labels},
		},
	}
}

func TestMatchesHelmValuesLabel_NewAndLegacy(t *testing.T) {
	newLbl := descriptor.Label{Name: LabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)}
	legacyLbl := descriptor.Label{Name: LegacyLabelHelmValuesFor, Value: json.RawMessage(`"mychart"`)}

	require.True(t, matchesHelmValuesLabel(resWithLabel("a", newLbl), "mychart"))
	require.True(t, matchesHelmValuesLabel(resWithLabel("b", legacyLbl), "mychart"))
	require.False(t, matchesHelmValuesLabel(resWithLabel("c", newLbl), "other"))
	require.False(t, matchesHelmValuesLabel(resWithLabel("d"), "mychart"))
	require.True(t, hasAnyHelmValuesLabel(resWithLabel("e", newLbl)))
	require.True(t, hasAnyHelmValuesLabel(resWithLabel("f", legacyLbl)))
	require.False(t, hasAnyHelmValuesLabel(resWithLabel("g")))
}
