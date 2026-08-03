// Package tree provides a reusable tree component for TUI views.
package tree

import (
	"strings"
)

// Node represents a single item in a tree. Domain packages set Data
// to hold their specific payload (e.g. *descriptor.Resource).
type Node struct {
	Label      string
	Depth      int
	Expanded   bool
	Loading    bool
	Expandable bool
	Children   []*Node
	Data       any
}

// Flatten returns a flat slice of visible nodes (respecting expand/collapse).
func Flatten(roots []*Node) []*Node {
	var result []*Node
	for _, root := range roots {
		flatten(root, &result)
	}
	return result
}

func flatten(n *Node, result *[]*Node) {
	*result = append(*result, n)
	if n.Expanded {
		for _, child := range n.Children {
			flatten(child, result)
		}
	}
}

// ContainsNode returns true if target is a descendant of parent.
func ContainsNode(parent, target *Node) bool {
	for _, child := range parent.Children {
		if child == target {
			return true
		}
		if ContainsNode(child, target) {
			return true
		}
	}
	return false
}

// RenderNode produces a single-line string for a tree node.
func RenderNode(n *Node, _ bool) string {
	indent := strings.Repeat("  ", n.Depth)

	var prefix string
	switch {
	case n.Loading:
		prefix = "~ "
	case n.Expandable && n.Expanded:
		prefix = "v "
	case n.Expandable:
		prefix = "> "
	default:
		prefix = "  "
	}

	return indent + prefix + n.Label
}
