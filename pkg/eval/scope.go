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
	// Parser is the file the expressions in this scope were parsed from. It
	// lives here rather than on every Expression, of which there are orders of
	// magnitude more.
	Parser  *p.Parser
	LowPrio bool
}

// Subscope nests a scope binding a set of names.
func (scope *Scope) Subscope(binds NixSet, lowPrio bool) *Scope {
	return &Scope{Binds: binds, LowPrio: lowPrio, Parent: scope, Parser: scope.Parser}
}

// Subscope1 nests a scope binding a single name.
func (scope *Scope) Subscope1(sym Sym, x *Expression) *Scope {
	return &Scope{sym: sym, expr: x, Parent: scope, Parser: scope.Parser}
}

// ForFile returns the scope bound to a parsed file, which is how the root
// scope of an evaluation is made from the shared DefaultScope.
func (scope *Scope) ForFile(pr *p.Parser) *Scope {
	s := *scope
	s.Parser = pr
	return &s
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
// `a."b".${c}` of a binding or a selection. The path node is passed in rather
// than wrapped in an expression, since nothing needs it afterwards.
func (scope *Scope) evalAttrPath(path *p.Node) []Sym {
	attrs := make([]Sym, len(path.Nodes))
	for i, c := range path.Nodes {
		attrs[i] = Intern(scope.attrName(c))
	}
	return attrs
}

// attrName evaluates one component of an attribute path to its name.
func (scope *Scope) attrName(n *p.Node) string {
	switch n.Type {
	case p.IDNode:
		return scope.Parser.TokenString(n.Tokens[0])
	case p.StringNode, p.IStringNode:
		return CoerceToString(scope.evalNode(n)).Content
	case p.InterpNode:
		return CoerceToString(scope.evalNode(n.Nodes[0])).Content
	default:
		throwf(ErrEval, "unsupported attribute name: %v", n.Type)
		return ""
	}
}
