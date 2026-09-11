package parser

import (
	"testing"

	"github.com/alecthomas/assert"
)

// TestMergeAttrPaths covers the parse-time merge of attribute paths. A path and
// a set literal that name the same attribute become one set, so the evaluator
// sees a single binding for `a` whose value holds both leaves.
func TestMergeAttrPaths(t *testing.T) {
	for _, src := range []string{
		`{ a.b = 1; a.c = 2; }`,
		`{ a = { b = 1; }; a.c = 2; }`,
		`{ a = { b = 1; }; a = { c = 2; }; }`,
		`{ a.b = 1; a = { c = 2; }; }`,
	} {
		p, err := ParseString(src)
		assert.NoError(t, err, src)

		set := p.Result
		assert.Equal(t, 1, len(set.Nodes), src)
		assert.Equal(t, "a", bindName(p, set.Nodes[0]), src)

		inner := set.Nodes[0].Nodes[1]
		assert.Equal(t, SetNode, inner.Type, src)
		assert.Equal(t, 2, len(inner.Nodes), src)
		assert.Equal(t, "b", bindName(p, inner.Nodes[0]), src)
		assert.Equal(t, "c", bindName(p, inner.Nodes[1]), src)
	}
}

// TestMergeAttrPathsKeepsComputed covers that a path with a computed component
// is not unfolded: its nesting is only known at evaluation time.
func TestMergeAttrPathsKeepsComputed(t *testing.T) {
	p, err := ParseString(`{ a.${builtins.toString 1} = 1; }`)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(p.Result.Nodes))
	assert.Equal(t, 2, len(p.Result.Nodes[0].Nodes[0].Nodes))
}

// TestMergeAttrPathsRejectsDuplicate covers that a repeat which cannot be
// merged is a parse error naming the full path.
func TestMergeAttrPathsRejectsDuplicate(t *testing.T) {
	for _, test := range [][2]string{
		{`{ a = 1; a = 2; }`, "a"},
		{`{ a = 1; a.b = 2; }`, "a"},
		{`{ a.b = 1; a.b = 2; }`, "a.b"},
		{`{ a = { b = 1; }; a.b = 2; }`, "a.b"},
		{`{ a = { b = 1; }; a = { b = 2; }; }`, "a.b"},
	} {
		_, err := ParseString(test[0])
		assert.Error(t, err, test[0])
		assert.Contains(t, err.Error(), "attribute '"+test[1]+"' already defined", test[0])
	}
}

// bindName is the static name of a binding whose path is a single identifier.
func bindName(p *Parser, b *Node) string {
	path := b.Nodes[0].Nodes
	return p.TokenString(path[len(path)-1].Tokens[0])
}
