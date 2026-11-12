package explore

import (
	"fmt"
	"slices"
	"strings"

	descriptor "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/runtime"

	"ext.ocm.software/tui/internal/component/tree"
	"ext.ocm.software/tui/internal/ui"
)

// item is the explorer payload of a tree node.
type item struct {
	// component and version identify the component version the node belongs to;
	// for a reference they point at the referenced component version.
	component, version string
	detail             string
	reference          bool
	resource           *descriptor.Resource
}

func data(n tree.Node) item {
	it, _ := n.Data.(item)
	return it
}

func componentNode(component string, versions ...string) tree.Node {
	root := tree.Node{ID: component, Label: component, Expanded: true, Data: item{component: component, detail: "Component: " + component}}
	for _, v := range versions {
		root.Children = append(root.Children, versionNode(root.ID, component, v))
	}
	return root
}

func versionNode(parentID, component, version string) tree.Node {
	return tree.Node{ID: parentID + "/" + version, Label: version, Lazy: true,
		Data: item{component: component, version: version, detail: "Version: " + version}}
}

// descriptorChildren builds the group nodes below a version or reference node.
func descriptorChildren(parent tree.Node, desc *descriptor.Descriptor) []tree.Node {
	cv := data(parent)
	c := &desc.Component
	var groups []tree.Node
	group := func(name string, children []tree.Node) {
		if len(children) > 0 {
			label := fmt.Sprintf("%s (%d)", name, len(children))
			groups = append(groups, tree.Node{ID: parent.ID + "/" + name, Label: label, Children: children, Data: item{component: cv.component, version: cv.version, detail: label}})
		}
	}
	leaf := func(group string, i int, label, detail string) tree.Node {
		return tree.Node{ID: fmt.Sprintf("%s/%s/%d", parent.ID, group, i), Label: label, Data: item{component: cv.component, version: cv.version, detail: detail}}
	}

	var nodes []tree.Node
	for i := range c.Resources {
		r := &c.Resources[i]
		n := leaf("Resources", i, elementLabel(r.Name, r.Type, r.ExtraIdentity), resourceDetail(r))
		it := data(n)
		it.resource = r
		n.Data = it
		nodes = append(nodes, n)
	}
	group("Resources", nodes)

	nodes = nil
	for i, s := range c.Sources {
		nodes = append(nodes, leaf("Sources", i, elementLabel(s.Name, s.Type, s.ExtraIdentity), sourceDetail(&s)))
	}
	group("Sources", nodes)

	nodes = nil
	for i, r := range c.References {
		n := leaf("References", i, fmt.Sprintf("%s → %s:%s", r.Name, r.Component, r.Version), referenceDetail(&r))
		n.Lazy, n.Data = true, item{component: r.Component, version: r.Version, detail: data(n).detail, reference: true}
		nodes = append(nodes, n)
	}
	group("References", nodes)

	nodes = nil
	for i, s := range desc.Signatures {
		nodes = append(nodes, leaf("Signatures", i, fmt.Sprintf("%s [%s]", s.Name, s.Signature.Algorithm), signatureDetail(&s)))
	}
	group("Signatures", nodes)

	nodes = nil
	for i, l := range c.Labels {
		nodes = append(nodes, leaf("Labels", i, fmt.Sprintf("%s = %s", l.Name, l.Value), labelsDetail([]descriptor.Label{l})))
	}
	group("Labels", nodes)
	return groups
}

// elementLabel builds a label like "ocmcli [executable] (arch=amd64, os=linux)".
func elementLabel(name, typ string, extra runtime.Identity) string {
	var parts []string
	for k, v := range extra {
		if k != "name" && k != "version" {
			parts = append(parts, k+"="+v)
		}
	}
	label := fmt.Sprintf("%s [%s]", name, typ)
	if len(parts) == 0 {
		return label
	}
	slices.Sort(parts)
	return label + " (" + strings.Join(parts, ", ") + ")"
}

// fields renders aligned "key: value" lines, skipping empty values.
func fields(kv ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			fmt.Fprintf(&b, "%s %s\n", ui.Dim.Render(fmt.Sprintf("%-11s", kv[i]+":")), kv[i+1])
		}
	}
	return b.String()
}

func digestDetail(d *descriptor.Digest) string {
	if d == nil || d.Value == "" {
		return ""
	}
	return "\n" + ui.Title.Render("Digest") + "\n" + fields("  Algorithm", d.HashAlgorithm, "  Normalisation", d.NormalisationAlgorithm, "  Value", d.Value)
}

func labelsDetail(labels []descriptor.Label) string {
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	for _, l := range labels {
		fmt.Fprintf(&b, "  %s: %s\n", l.Name, l.Value)
	}
	return "\n" + ui.Title.Render("Labels") + "\n" + b.String()
}

func versionDetail(d *descriptor.Descriptor) string {
	c := &d.Component
	return fields("Name", c.Name, "Version", c.Version, "Provider", c.Provider.Name, "Created", c.CreationTime, "Schema", d.Meta.Version) + "\n" +
		fields("Resources", fmt.Sprint(len(c.Resources)), "Sources", fmt.Sprint(len(c.Sources)),
			"References", fmt.Sprint(len(c.References)), "Signatures", fmt.Sprint(len(d.Signatures)))
}

func resourceDetail(r *descriptor.Resource) string {
	return fields("Name", r.Name, "Version", r.Version, "Type", r.Type, "Relation", string(r.Relation)) + digestDetail(r.Digest) + labelsDetail(r.Labels)
}

func sourceDetail(s *descriptor.Source) string {
	return fields("Name", s.Name, "Version", s.Version, "Type", s.Type) + labelsDetail(s.Labels)
}

func referenceDetail(r *descriptor.Reference) string {
	return fields("Name", r.Name, "Component", r.Component, "Version", r.Version) + digestDetail(&r.Digest) + labelsDetail(r.Labels)
}

func signatureDetail(s *descriptor.Signature) string {
	return fields("Name", s.Name, "Algorithm", s.Signature.Algorithm, "MediaType", s.Signature.MediaType, "Issuer", s.Signature.Issuer) + digestDetail(&s.Digest)
}
