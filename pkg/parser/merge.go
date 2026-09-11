package parser

import "fmt"

// Nix merges attribute-path bindings while parsing, before evaluating: a set
// that several bindings define through a common prefix is one set, and a
// repeated leaf is a duplicate. So `{ a.b = 1; a = { c = 2; }; }` is
// `{ a = { b = 1; c = 2; }; }`, and `{ a = 1; a.b = 2; }` is an error.
//
// The grammar here builds a flat binding list, and the evaluator would need to
// know which bindings name sets to merge them. It is simpler and closer to Nix
// to canonicalise the tree once, here: every static path is unfolded into
// nested sets, and bindings that name the same attribute with sets are merged.
// A repeat that cannot be merged is reported, as Nix's parser reports it,
// rather than left for the evaluator to find when it happens to force the set.

// normalize rewrites the tree bottom-up so that every set has canonical
// bindings.
func (p *Parser) normalize(n *Node) {
	for _, c := range n.Nodes {
		p.normalize(c)
	}
	switch n.Type {
	case SetNode, RecSetNode, BindsNode:
		n.Nodes = p.mergeBinds(n.Nodes, "")
	}
}

// mergeBinds unfolds the attribute paths in one binding list and merges the
// bindings that name the same static attribute with two sets. prefix is the
// path to this binding list, used to name a duplicate in the error.
func (p *Parser) mergeBinds(bindNodes []*Node, prefix string) []*Node {
	if len(p.errors) > 0 {
		return bindNodes
	}
	out := make([]*Node, 0, len(bindNodes))
	index := map[string]int{}
	for _, c := range bindNodes {
		c = p.unfold(c)
		name, ok := staticBindName(p, c)
		if !ok {
			out = append(out, c)
			continue
		}
		i, seen := index[name]
		if !seen {
			index[name] = len(out)
			out = append(out, c)
			continue
		}
		if merged, ok := p.mergeSets(out[i], c, prefix+name+"."); ok {
			out[i] = merged
			continue
		}
		p.dupAttr(c, prefix+name)
		out = append(out, c)
	}
	return out
}

// staticBindName is the name of a binding whose path is a single static
// identifier, which is the only kind that can be merged here. Everything else,
// including a computed name, is left for the evaluator.
func staticBindName(p *Parser, c *Node) (string, bool) {
	if c.Type != BindNode {
		return "", false
	}
	path := c.Nodes[0].Nodes
	if len(path) != 1 || path[0].Type != IDNode {
		return "", false
	}
	return p.TokenString(path[0].Tokens[0]), true
}

// unfold turns `a.b.c = e` into `a = { b = { c = e; }; }`, which is what Nix's
// parser does as it reads the path. A path with a computed component is left
// alone: its nesting is only known at evaluation time.
func (p *Parser) unfold(c *Node) *Node {
	if c.Type != BindNode {
		return c
	}
	path := c.Nodes[0].Nodes
	if len(path) <= 1 {
		return c
	}
	for _, comp := range path {
		if comp.Type != IDNode {
			return c
		}
	}
	rhs := c.Nodes[1]
	for i := len(path) - 1; i >= 1; i-- {
		inner := p.NewNode(BindNode).N2(p.NewNode(AttrPathNode).N1(path[i]), rhs)
		set := p.NewNode(SetNode)
		set.Nodes = append(set.Nodes, inner)
		rhs = set
	}
	return p.NewNode(BindNode).N2(p.NewNode(AttrPathNode).N1(path[0]), rhs)
}

// mergeSets merges two bindings that name the same attribute with sets,
// returning the merged binding. When either value is not a set they must not be
// merged, and the caller reports the duplicate.
func (p *Parser) mergeSets(a, b *Node, innerPrefix string) (*Node, bool) {
	as, bs := a.Nodes[1], b.Nodes[1]
	if !isSetLiteral(as) || !isSetLiteral(bs) {
		return nil, false
	}
	combined := make([]*Node, 0, len(as.Nodes)+len(bs.Nodes))
	combined = append(combined, as.Nodes...)
	combined = append(combined, bs.Nodes...)
	as.Nodes = p.mergeBinds(combined, innerPrefix)
	// Nix keeps the first binding's shape, including whether it is recursive.
	return a, true
}

// dupAttr records a repeated attribute under path, at the position of the
// binding that repeats it.
func (p *Parser) dupAttr(c *Node, path string) {
	pos := p.NodePos(c.Nodes[0])
	p.errors = append(p.errors, &ParserError{
		Pos:  pos,
		Line: p.SourceLine(pos),
		Desc: fmt.Sprintf("attribute '%s' already defined", path),
	})
}

func isSetLiteral(n *Node) bool { return n.Type == SetNode || n.Type == RecSetNode }
