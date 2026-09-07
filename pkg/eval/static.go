package eval

import (
	"slices"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// Static facts about a syntax node — the value of a literal, the interned name
// of an identifier — do not change between evaluations, but a node inside a
// function body is evaluated once per call. This file caches them.
//
// The cache is a sparse array keyed by the node's dense ID, in fixed-size
// pages, so a lookup is two array indexes and no hashing. Only the pages that
// are touched are allocated, which matters when a large file is parsed and a
// small part of it evaluated. The design is taken from the TypeScript Go
// compiler's core.PagedLinkStore.
const (
	staticPageShift = 8
	staticPageSize  = 1 << staticPageShift
	staticPageMask  = staticPageSize - 1
)

// static is what is known about a node without evaluating it in a scope.
type static struct {
	// val is the value of a literal: a number, a path, or a string with no
	// interpolation in it.
	val NixValue
	// sym is the interned name of an identifier or of an attribute-path
	// component. The zero Sym is the empty name, which no identifier has, so
	// it doubles as "not computed yet".
	sym Sym
}

type staticStore struct {
	pages []*[staticPageSize]static
}

// get returns the entry for a node, creating its page on first use.
func (s *staticStore) get(id uint32) *static {
	page := int(id >> staticPageShift)
	if page >= len(s.pages) {
		// Grow rounds the capacity up to a size class, so the pages of a file
		// parsed all at once cost one allocation between them.
		s.pages = slices.Grow(s.pages, page+1-len(s.pages))[:page+1]
	}
	pg := s.pages[page]
	if pg == nil {
		pg = new([staticPageSize]static)
		s.pages[page] = pg
	}
	return &pg[id&staticPageMask]
}

// file is the state an evaluation keeps per parsed file: the source itself and
// what has been worked out about its nodes. Scopes hold a pointer to it, so an
// expression reaches both through its scope and neither costs a field of its
// own.
type file struct {
	parser *p.Parser
	static staticStore
}

// literal returns the value of a literal node, computing it at most once.
func (scope *Scope) literal(n *p.Node, compute func(string) NixValue) NixValue {
	e := scope.file.static.get(n.ID)
	if e.val == nil {
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
