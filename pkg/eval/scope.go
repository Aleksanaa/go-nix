package eval

import (
	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// Scope is a chain of bindings an identifier is looked up in.
//
// A scope binds either a whole set of names (a `let`, a recursive set, a
// function called with formal arguments) or a single one. The single case is
// the plain `arg: body` call, which is common enough — every curried function
// makes one per argument — that giving it a Go map of one entry showed up as
// the largest single source of allocation in a profile.
//
// `with` introduces a low-priority scope: names it provides are only used once
// the whole chain has been searched for ordinary (lexical) bindings, and a
// nearer `with` shadows a farther one.
type Scope struct {
	Binds NixSet // nil when this scope binds a single name

	sym  Sym         // the bound name, when Binds is nil
	expr *Expression // its value, and the marker for the single-name case

	Parent *Scope
	// file is the source these expressions were parsed from, together with what
	// has been worked out about its nodes. It lives here rather than on every
	// Expression, of which there are orders of magnitude more.
	file    *file
	LowPrio bool
}

// Subscope nests a scope binding a set of names.
func (scope *Scope) Subscope(binds NixSet, lowPrio bool) *Scope {
	return &Scope{Binds: binds, LowPrio: lowPrio, Parent: scope, file: scope.file}
}

// Subscope1 nests a scope binding a single name.
func (scope *Scope) Subscope1(sym Sym, x *Expression) *Scope {
	return &Scope{sym: sym, expr: x, Parent: scope, file: scope.file}
}

// ForFile returns the scope bound to a parsed file, which is how the root
// scope of an evaluation is made from the shared DefaultScope.
func (scope *Scope) ForFile(pr *p.Parser) *Scope {
	s := *scope
	s.file = &file{parser: pr}
	return &s
}

// parser is the source the expressions in this scope were parsed from, or nil
// for a scope with no syntax behind it.
func (scope *Scope) parser() *p.Parser {
	if scope == nil || scope.file == nil {
		return nil
	}
	return scope.file.parser
}

// Lookup finds sym, searching lexical bindings first and `with` bindings only
// afterwards.
func (scope *Scope) Lookup(sym Sym) (*Expression, bool) {
	for _, lowPrio := range [2]bool{false, true} {
		for s := scope; s != nil; s = s.Parent {
			if s.LowPrio != lowPrio {
				continue
			}
			if s.expr != nil {
				if s.sym == sym {
					return s.expr, true
				}
				continue
			}
			if x, ok := s.Binds[sym]; ok {
				return x, true
			}
		}
	}
	return nil, false
}

// evalNode evaluates a node in this scope, with no surrounding expression to
// take the scope from.
func (scope *Scope) evalNode(n *p.Node) NixValue {
	y := Expression{Scope: scope, Node: n}
	return y.Eval()
}

// evalAttrPath evaluates the names of an attribute path, such as the
// `a."b".` of a binding or a selection. The path node is passed in rather
// than wrapped in an expression, since nothing needs it afterwards.
func (scope *Scope) evalAttrPath(path *p.Node) []Sym {
	attrs := make([]Sym, len(path.Nodes))
	for i, c := range path.Nodes {
		attrs[i] = scope.attrSym(c)
	}
	return attrs
}

// attrSym evaluates one component of an attribute path to its interned name.
// A plain identifier is interned once and kept against the node; a computed
// one has to be evaluated every time.
func (scope *Scope) attrSym(n *p.Node) Sym {
	switch n.Type {
	case p.IDNode:
		return scope.name(n)
	case p.StringNode, p.IStringNode:
		return Intern(CoerceToString(scope.evalNode(n)).Content)
	case p.InterpNode:
		return Intern(CoerceToString(scope.evalNode(n.Nodes[0])).Content)
	default:
		throwf(ErrEval, "unsupported attribute name: %v", n.Type)
		return 0
	}
}

// Names lists every name this scope makes visible, nearest binding first and
// without repetition.
//
// It exists for the REPL's completion, which is outside this package and so
// has no other way to see what a chain of scopes holds.
func (scope *Scope) Names() []string {
	var names []string
	seen := map[Sym]bool{}
	add := func(sym Sym) {
		if !seen[sym] {
			seen[sym] = true
			names = append(names, sym.String())
		}
	}
	for s := scope; s != nil; s = s.Parent {
		if s.expr != nil {
			add(s.sym)
			continue
		}
		for _, sym := range s.Binds.Keys() {
			add(sym)
		}
	}
	return names
}
