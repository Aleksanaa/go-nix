package eval

import (
	"slices"
	"strconv"
	"sync/atomic"

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
	// attrs is what an attribute path of plain identifiers names, which does
	// not change between evaluations.
	attrs []Sym
	// attrSym is the name of the attribute whose value this node is, for the
	// backtrace frame that says which attribute failed.
	//
	// It is the one thing here that evaluation can still write: an attribute
	// whose name is itself computed is only named once the group is evaluated.
	// Atomic so that workers evaluating the same group agree, and because a
	// name settled by the pass is never written again — the store is guarded
	// by a load, so the ordinary case is a read.
	attrSym atomic.Int32
	// bad is why the pass could not make sense of this node, raised if and
	// when the node is evaluated so that the failure keeps its backtrace.
	bad string
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
	// sealed says the pass has finished and nothing may write here again.
	// Evaluation only reads what is known about the syntax, so that several
	// workers can read it at once; this is what catches a write that slips
	// back in.
	sealed bool
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
	f := &file{parser: pr, static: staticStore{entries: make([]static, pr.NodeCount())}}
	f.prepare()
	return f
}

// The value of each kind of literal, computed at most once per node.

func uriLiteral(w *worker, s string) NixValue { return String(s) }

// TODO: resolve relative to the file being evaluated, and <lookup> paths
// through NIX_PATH.
func pathLiteral(w *worker, s string) NixValue { return PathValue(&NixPath{Root: "/", Path: s}) }

func floatLiteral(w *worker, s string) NixValue {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		w.throwf(ErrSyntax, "invalid float %q", s)
	}
	return Float(val)
}

func intLiteral(w *worker, s string) NixValue {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		w.throwf(ErrSyntax, "invalid integer %q", s)
	}
	return Int(val)
}

// literalValue returns the value of a literal node, computing it at most
// once. It is for reading a literal with no expression around it, where there
// is no thunk to hold the value: an operand of an arithmetic expression is
// usually a literal or a name, and going through the whole force machinery
// for a number was a measurable cost on call-heavy workloads.
func (scope *Scope) literalValue(w *worker, n *p.Node) (NixValue, bool) {
	switch n.Type {
	case p.IntNode:
		return scope.literal(w, n, intLiteral), true
	case p.FloatNode:
		return scope.literal(w, n, floatLiteral), true
	case p.PathNode:
		return scope.literal(w, n, pathLiteral), true
	case p.URINode:
		return scope.literal(w, n, uriLiteral), true
	}
	return NixValue{}, false
}

// literal returns the value of a literal node, which the pass worked out
// before the evaluation started. A literal the pass could not take — a number
// out of range — is raised here, where the evaluation has a backtrace to
// attach, rather than when the file was loaded.
func (scope *Scope) literal(w *worker, n *p.Node, compute func(*worker, string) NixValue) NixValue {
	e := scope.file.static.get(n.ID)
	if e.bad != "" {
		w.throwf(ErrSyntax, "%s", e.bad)
	}
	return e.val
}

// name returns the interned name of an identifier node, interning it at most
// once.
func (scope *Scope) name(n *p.Node) Sym {
	return scope.file.static.get(n.ID).sym
}

// literalExpr returns a literal node as an already evaluated expression. A
// literal is the same value however often it is reached, so one expression
// serves every use of the node, and passing one as an argument costs nothing.
func (scope *Scope) literalExpr(w *worker, n *p.Node) *Expression {
	e := scope.file.static.get(n.ID)
	if e.bad != "" {
		w.throwf(ErrSyntax, "%s", e.bad)
	}
	return e.expr
}

// cache records the value of a literal that the pass works out, and refuses to
// once the pass is done. The pass evaluates every string with nothing
// interpolated into it, so evaluation never reaches this; if it ever does, the
// cache is being written while workers may be reading it, and saying so here
// is better than a race that only shows up under load.
func (e *static) cache(f *file, val NixValue) {
	if f.static.sealed {
		panic("eval: the cache against the syntax was written while evaluating")
	}
	e.val = val
}
