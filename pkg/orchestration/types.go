package orchestration

import "time"

// Kind identifies the orchestration node category.
type Kind string

const (
	KindTask        Kind = "task"
	KindSequence    Kind = "sequence"
	KindParallel    Kind = "parallel"
	KindConditional Kind = "conditional"
)

// Node is the declarative unit shared between orchestration planning and the
// runtime adapters that will execute it in later tasks.
type Node struct {
	Kind     Kind
	Name     string
	Nodes    []Node
	Branches []Branch
	Default  *Node
	Retry    *RetrySpec
	Result   *ResultEnvelope
	Metadata map[string]any
}

// Branch declares either a named fan-out branch or a conditional path.
type Branch struct {
	Name string
	When string
	Node Node
}

// RetrySpec declares how a node may be retried by a future executor.
type RetrySpec struct {
	MaxAttempts int
	Backoff     time.Duration
}

// ResultEnvelope is the minimal structured output shape shared by the runtime
// response adapter and orchestration fan-in nodes.
type ResultEnvelope struct {
	Text       string
	Structured map[string]any
	Branches   map[string]ResultEnvelope
	Metadata   map[string]any
}

// Sequence composes child nodes in order.
func Sequence(nodes ...Node) Node {
	return Node{
		Kind:  KindSequence,
		Nodes: cloneNodes(nodes),
	}
}

// Parallel declares fan-out branches with named fan-in aggregation.
func Parallel(branches ...Branch) Node {
	return Node{
		Kind:     KindParallel,
		Branches: cloneBranches(branches),
		Result: &ResultEnvelope{
			Branches: map[string]ResultEnvelope{},
		},
	}
}

// Conditional declares ordered predicate branches plus an optional default path.
func Conditional(branches ...Branch) Node {
	node := Node{Kind: KindConditional}
	for _, branch := range branches {
		clone := cloneNode(branch.Node)
		if branch.When == "" {
			node.Default = &clone
			continue
		}
		node.Branches = append(node.Branches, Branch{
			Name: branch.Name,
			When: branch.When,
			Node: clone,
		})
	}
	return node
}

// FanOut declares a named parallel branch.
func FanOut(name string, node Node) Branch {
	return Branch{Name: name, Node: cloneNode(node)}
}

// When declares a conditional branch.
func When(expr string, node Node) Branch {
	return Branch{When: expr, Node: cloneNode(node)}
}

// Otherwise declares the default conditional branch.
func Otherwise(node Node) Branch {
	return Branch{Node: cloneNode(node)}
}

// WithRetry attaches a retry declaration to a copied node.
func (n Node) WithRetry(spec RetrySpec) Node {
	n = cloneNode(n)
	n.Retry = &RetrySpec{
		MaxAttempts: spec.MaxAttempts,
		Backoff:     spec.Backoff,
	}
	return n
}

func cloneNodes(nodes []Node) []Node {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, cloneNode(node))
	}
	return out
}

func cloneBranches(branches []Branch) []Branch {
	if len(branches) == 0 {
		return nil
	}
	out := make([]Branch, 0, len(branches))
	for _, branch := range branches {
		out = append(out, Branch{
			Name: branch.Name,
			When: branch.When,
			Node: cloneNode(branch.Node),
		})
	}
	return out
}

func cloneNode(node Node) Node {
	cloned := node
	cloned.Nodes = cloneNodes(node.Nodes)
	cloned.Branches = cloneBranches(node.Branches)
	if node.Default != nil {
		defaultNode := cloneNode(*node.Default)
		cloned.Default = &defaultNode
	}
	if node.Retry != nil {
		cloned.Retry = &RetrySpec{
			MaxAttempts: node.Retry.MaxAttempts,
			Backoff:     node.Retry.Backoff,
		}
	}
	if node.Result != nil {
		cloned.Result = cloneEnvelope(node.Result)
	}
	if len(node.Metadata) > 0 {
		cloned.Metadata = make(map[string]any, len(node.Metadata))
		for k, v := range node.Metadata {
			cloned.Metadata[k] = v
		}
	}
	return cloned
}

func cloneEnvelope(in *ResultEnvelope) *ResultEnvelope {
	if in == nil {
		return nil
	}
	out := &ResultEnvelope{
		Text: in.Text,
	}
	if len(in.Structured) > 0 {
		out.Structured = make(map[string]any, len(in.Structured))
		for k, v := range in.Structured {
			out.Structured[k] = v
		}
	}
	if len(in.Branches) > 0 {
		out.Branches = make(map[string]ResultEnvelope, len(in.Branches))
		for k, v := range in.Branches {
			out.Branches[k] = v
		}
	} else if in.Branches != nil {
		out.Branches = map[string]ResultEnvelope{}
	}
	if len(in.Metadata) > 0 {
		out.Metadata = make(map[string]any, len(in.Metadata))
		for k, v := range in.Metadata {
			out.Metadata[k] = v
		}
	}
	return out
}
