package eval

import (
	"slices"
	"strconv"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// Static facts about a syntax node — the value of a literal, the interned name
// of an identifier — do not change between evaluations, but a node inside a
// function body is evaluated once per call. This file caches them.
//
// The cache is a flat array keyed by the node's dense ID, so a lookup is one
// bounds check and an index, with no hashing and no indirection. The parser
// says how many nodes it made, so the array is the right size from the start;
// it was pages of a sparse array before, and reaching through the page table
// was 11% of the evaluator on a call-heavy workload.

// static is what is known about a node without evaluating it in a scope.
type static struct {
	// val is the value of a literal: a number, a path, or a string with no
	// interpolation in it.
	val NixValue
	// expr is a literal node as an expression, for where one is passed on
	// rather than evaluated.
	expr *Expression
	// lambda is what a function node binds and where its body is, neither of
	// which depends on the scope a closure over it is made in.
	lambda *lambdaInfo
	// hops is one more than the number of scopes between the one an
	// identifier is evaluated in and the one that binds it, or zero when that
	// is not settled yet.
	hops int32
	// slot is one more than the position of the name in the scope hops leads
	// to, or zero when that scope binds a single name.
	slot int32
	// attrs is what an attribute path of plain identifiers names, which does
	// not change between evaluations.
	attrs []Sym
	// attrSym is the name of the attribute whose value this node is, for the
	// backtrace frame that says which attribute failed.
	attrSym Sym
	// owner is the function a body node belongs to, which is where a call's
	// frame points.
	owner *p.Node
	// sym is the interned name of an identifier or of an attribute-path
	// component. The zero Sym is the empty name, which no identifier has, so
	// it doubles as "not computed yet".
	sym Sym
}

type staticStore struct {
	entries []static
}

// get returns the entry for a node.
func (s *staticStore) get(id uint32) *static {
	if int(id) >= len(s.entries) {
		s.grow(id)
	}
	return &s.entries[id]
}

// grow makes room for a node the array does not cover yet, which happens once
// per file: the first node evaluated sizes it for the whole parse.
func (s *staticStore) grow(id uint32) {
	s.entries = slices.Grow(s.entries, int(id)+1-len(s.entries))[:id+1]
}

// file is the state an evaluation keeps per parsed file: the source itself and
// what has been worked out about its nodes. Scopes hold a pointer to it, so an
// expression reaches both through its scope and neither costs a field of its
// own.
type file struct {
	parser *p.Parser
	static staticStore
}

// newFile binds a parse to the facts worked out about its nodes. The parser
// counted them, so the array they go in is allocated once, at the right size.
func newFile(pr *p.Parser) *file {
	return &file{parser: pr, static: staticStore{entries: make([]static, pr.NodeCount())}}
}

// The value of each kind of literal, computed at most once per node.

func uriLiteral(s string) NixValue { return String(s) }

// TODO: resolve relative to the file being evaluated, and <lookup> paths
// through NIX_PATH.
func pathLiteral(s string) NixValue { return PathValue(&NixPath{Root: "/", Path: s}) }

func floatLiteral(s string) NixValue {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		throwf(ErrSyntax, "invalid float %q", s)
	}
	return Float(val)
}

func intLiteral(s string) NixValue {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		throwf(ErrSyntax, "invalid integer %q", s)
	}
	return Int(val)
}

// literalValue returns the value of a literal node, computing it at most
// once. It is for reading a literal with no expression around it, where there
// is no thunk to hold the value: an operand of an arithmetic expression is
// usually a literal or a name, and going through the whole force machinery
// for a number was a measurable cost on call-heavy workloads.
func (scope *Scope) literalValue(n *p.Node) (NixValue, bool) {
	switch n.Type {
	case p.IntNode:
		return scope.literal(n, intLiteral), true
	case p.FloatNode:
		return scope.literal(n, floatLiteral), true
	case p.PathNode:
		return scope.literal(n, pathLiteral), true
	case p.URINode:
		return scope.literal(n, uriLiteral), true
	}
	return NixValue{}, false
}

// literal returns the value of a literal node, computing it at most once.
func (scope *Scope) literal(n *p.Node, compute func(string) NixValue) NixValue {
	e := scope.file.static.get(n.ID)
	if e.val.IsNone() {
		e.val = compute(scope.file.parser.TokenString(n.Tokens[0]))
	}
	return e.val
}

// name returns the interned name of an identifier node, interning it at most
// once.
func (scope *Scope) name(n *p.Node) Sym {
	e := scope.file.static.get(n.ID)
	if e.sym == 0 {
		e.sym = Intern(scope.file.parser.TokenString(n.Tokens[0]))
	}
	return e.sym
}

// literalExpr returns a literal node as an already evaluated expression. A
// literal is the same value however often it is reached, so one expression
// serves every use of the node, and passing one as an argument costs nothing.
func (scope *Scope) literalExpr(n *p.Node) *Expression {
	e := scope.file.static.get(n.ID)
	if e.expr == nil {
		e.expr = value(scope.evalNode(n))
	}
	return e.expr
}
