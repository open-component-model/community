package helmvalues

import (
	"encoding/json"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

const (
	// LabelHelmValuesFor is the compliant community label key (NAMING.md:
	// ext.ocm.software namespace). Preferred.
	LabelHelmValuesFor = "ext.ocm.software/helm.values-for"
	// LegacyLabelHelmValuesFor is the pre-contribution key, still read for
	// backward compatibility with already-published components.
	LegacyLabelHelmValuesFor = "opendefense.cloud/helm/values-for"
)

func isHelmValuesLabelKey(name string) bool {
	return name == LabelHelmValuesFor || name == LegacyLabelHelmValuesFor
}

// matchesHelmValuesLabel reports whether res carries a helm-values label
// (new or legacy key) whose value equals chart.
func matchesHelmValuesLabel(res descriptor.Resource, chart string) bool {
	for _, l := range res.Labels {
		if isHelmValuesLabelKey(l.Name) && labelValueEquals(l, chart) {
			return true
		}
	}
	return false
}

// hasAnyHelmValuesLabel reports whether res carries any helm-values label,
// regardless of value.
func hasAnyHelmValuesLabel(res descriptor.Resource) bool {
	for _, l := range res.Labels {
		if isHelmValuesLabelKey(l.Name) {
			return true
		}
	}
	return false
}

func labelValueEquals(l descriptor.Label, target string) bool {
	// v2 label values are always json.RawMessage; a string value is JSON-quoted.
	var s string
	if err := json.Unmarshal(l.Value, &s); err == nil {
		return s == target
	}
	return string(l.Value) == target
}
