package helmvalues

import (
	"encoding/json"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
)

const (
	// LabelHelmValuesFor is the preferred label key identifying a Helm values
	// template, in the ext.ocm.software community namespace (see NAMING.md).
	LabelHelmValuesFor = "ext.ocm.software/helm.values-for"
	// LegacyLabelHelmValuesFor is an older key still recognized for backward
	// compatibility with already-published components.
	LegacyLabelHelmValuesFor = "opendefense.cloud/helm/values-for"
)

func isHelmValuesLabelKey(name string) bool {
	return name == LabelHelmValuesFor || name == LegacyLabelHelmValuesFor
}

// matchesHelmValuesLabel reports whether res carries a helm-values label
// (under either recognized key) whose value equals chart.
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
	// Label values are always json.RawMessage; a string value is JSON-quoted.
	var s string
	if err := json.Unmarshal(l.Value, &s); err == nil {
		return s == target
	}
	return string(l.Value) == target
}
