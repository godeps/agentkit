package artifact

// LineageEdge captures a derivation relationship between two artifacts.
type LineageEdge struct {
	Parent    ArtifactRef `json:"parent"`
	Child     ArtifactRef `json:"child"`
	Operation string      `json:"operation,omitempty"`
}

// LineageGraph stores derivation edges between runtime artifacts.
type LineageGraph struct {
	Edges []LineageEdge `json:"edges,omitempty"`
}

// AddEdge records a derivation from parent to child.
func (g *LineageGraph) AddEdge(parent, child ArtifactRef, operation string) {
	g.Edges = append(g.Edges, LineageEdge{
		Parent:    parent,
		Child:     child,
		Operation: operation,
	})
}

// ChildrenOf returns the direct derived artifacts for a given parent.
func (g LineageGraph) ChildrenOf(parent ArtifactRef) []ArtifactRef {
	var out []ArtifactRef
	for _, edge := range g.Edges {
		if edge.Parent == parent {
			out = append(out, edge.Child)
		}
	}
	return out
}

// AncestorsOf returns the provenance chain from the direct parent upward.
func (g LineageGraph) AncestorsOf(child ArtifactRef) []ArtifactRef {
	var out []ArtifactRef
	seen := map[ArtifactRef]struct{}{}
	current := child
	for {
		parent, ok := g.parentOf(current)
		if !ok {
			return out
		}
		if _, exists := seen[parent]; exists {
			return out
		}
		seen[parent] = struct{}{}
		out = append(out, parent)
		current = parent
	}
}

func (g LineageGraph) parentOf(child ArtifactRef) (ArtifactRef, bool) {
	for i := len(g.Edges) - 1; i >= 0; i-- {
		if g.Edges[i].Child == child {
			return g.Edges[i].Parent, true
		}
	}
	return ArtifactRef{}, false
}
