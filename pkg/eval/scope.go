package eval

import (
	"unsafe"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// Scope is a chain of bindings an identifier is looked up in.
//
// A scope binds either a whole set of names (a `let`, a recursive set, a
// function called with formal arguments) or a single one. The single case is
// the plain `arg: body` call, which is common enough — every curried function
// makes one per argument — that giving it a set of one entry showed up as the
// largest single source of allocation in a profile.
//
// Since it is never both, the two share a word, which keeps a scope to 32
// bytes: after thunks, scopes are what an evaluation allocates most of.
//
// `with` introduces a low-priority scope: names it provides are only used once
// the whole chain has been searched for ordinary (lexical) bindings, and a
// nearer `with` shadows a farther one.
type Scope struct {
	// bound is the set of names this scope binds, or the expression the single
	// name stands for. sym says which: it is zero for a set.
	bound  unsafe.Pointer
	Parent *Scope
	// file is the source these expressions were parsed from, together with what
	// has been worked out about its nodes. It lives here rather than on every
	// Expression, of which there are orders of magnitude more.
	file    *file
	sym     Sym
	LowPrio bool
}

// binds is the set of names this scope binds, or nil when it binds one name.
func (s *Scope) binds() NixSet {
	if s.sym != 0 {
		return nil
	}
	return (*AttrSet)(s.bound)
}

// single is the expression this scope's one name stands for, or nil when it
// binds a set of names.
func (s *Scope) single() *Expression {
	if s.sym == 0 {
		return nil
	}
	return (*Expression)(s.bound)
}

// scopeSlabSize is how many scopes are allocated at a time. A scope is only
// ever reached from the one nested inside it, so a block of them is retained
// exactly as long as its liveliest member — the same trade as exprSlabSize,
// but a cheaper one, since a scope chain dies together.
const scopeSlabSize = 256

// scopeSlab is the block currently being handed out. Evaluation is
// single-goroutine, like the symbol table and the evaluation stack.
var scopeSlab []Scope

// newScope returns a zeroed scope from the block being handed out.
func newScope() *Scope {
	if scopeSlabSize <= 1 {
		return new(Scope)
	}
	if len(scopeSlab) == 0 {
		scopeSlab = make([]Scope, scopeSlabSize)
	}
	s := &scopeSlab[0]
	scopeSlab = scopeSlab[1:]
	return s
}

// Subscope nests a scope binding a set of names.
func (scope *Scope) Subscope(binds NixSet, lowPrio bool) *Scope {
	s := newScope()
	*s = Scope{bound: unsafe.Pointer(binds), LowPrio: lowPrio, Parent: scope, file: scope.file}
	return s
}

// Subscope1 nests a scope binding a single name.
func (scope *Scope) Subscope1(sym Sym, x *Expression) *Scope {
	s := newScope()
	*s = Scope{sym: sym, bound: unsafe.Pointer(x), Parent: scope, file: scope.file}
	return s
}

// ForFile returns the scope bound to a parsed file, which is how the root
// scope of an evaluation is made from the shared DefaultScope.
func (scope *Scope) ForFile(pr *p.Parser) *Scope {
	s := *scope
	s.file = newFile(pr)
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
//
// One walk answers both: a lexical binding anywhere in the chain beats every
// `with`, so the nearest `with` match is remembered and only used once the
// walk has finished without finding a lexical one.
func (scope *Scope) Lookup(sym Sym) (*Expression, bool) {
	x, _, _, ok := scope.lookupFrom(sym)
	return x, ok
}

// lookupFrom is Lookup, also reporting where a lexical binding was found: how
// many scopes had to be skipped, and which slot of that scope holds it. A
// `with` reports neither: what its set holds is not decided until it is
// evaluated, and a nearer `with` shadows a farther one, so there is nothing
// about it worth remembering.
func (scope *Scope) lookupFrom(sym Sym) (x *Expression, hops, slot int32, ok bool) {
	var with *Expression
	for s := scope; s != nil; s, hops = s.Parent, hops+1 {
		if s.LowPrio {
			if with == nil {
				if y, found := (*AttrSet)(s.bound).Get(sym); found {
					with = y
				}
			}
			continue
		}
		if s.sym != 0 {
			if s.sym == sym {
				return (*Expression)(s.bound), hops, -1, true
			}
			continue
		}
		if set := (*AttrSet)(s.bound); set != nil {
			if y, slot, found := set.getSlot(sym); found {
				return y, hops, slot, true
			}
		}
	}
	// A `with` reports no place: -1 says there is nothing to remember.
	return with, -1, -1, with != nil
}

// lookupNode finds what an identifier node refers to, and interns its name.
//
// Which scope holds a name is decided by the syntax: the same identifier node,
// evaluated again, is reached through a chain of scopes of the same shape. So
// the number of scopes to skip is remembered against the node and the next
// evaluation jumps straight to it, instead of probing a map at every level on
// the way. Nix settles this once and for all at parse time; this arrives at
// the same place without a pass of its own, and checks the name it lands on,
// so that a chain of a shape it did not expect costs a search rather than a
// wrong answer.
func (scope *Scope) lookupNode(n *p.Node) (Sym, *Expression, bool) {
	e := scope.file.static.get(n.ID)
	if e.sym == 0 {
		e.sym = Intern(scope.file.parser.TokenString(n.Tokens[0]))
	}
	sym := e.sym
	if e.hops < 0 {
		// Nothing in the chain binds this name lexically, which the syntax
		// decides once and for all, so only a `with` can have it and the
		// nearest one wins. Nix marks such a name the same way.
		for s := scope; s != nil; s = s.Parent {
			if s.LowPrio {
				if x, ok := (*AttrSet)(s.bound).Get(sym); ok {
					return sym, x, true
				}
			}
		}
		return sym, nil, false
	}
	if e.hops > 0 {
		s := scope
		for i := e.hops - 1; i > 0 && s != nil; i-- {
			s = s.Parent
		}
		if s != nil {
			if s.sym != 0 {
				if s.sym == sym {
					return sym, (*Expression)(s.bound), true
				}
			} else if x, ok := (*AttrSet)(s.bound).at(e.slot-1, sym); ok {
				// The name was in this slot last time and still is, which
				// is the whole lookup: two loads and a compare.
				return sym, x, true
			}
		}
	}
	x, hops, slot, ok := scope.lookupFrom(sym)
	if hops >= 0 {
		e.hops, e.slot = hops+1, slot+1
	} else {
		// Either a `with` provided it or nothing did; both mean there is no
		// lexical binding to find next time.
		e.hops = -1
	}
	return sym, x, ok
}

// evalNode evaluates a node in this scope, with no surrounding expression to
// take the scope from.
func (scope *Scope) evalNode(n *p.Node) NixValue {
	// Reading a name is the value of the thunk it is bound to. An expression
	// of its own would mean forcing twice — once for the read, once for the
	// binding — and a backtrace frame that says nothing the binding's own
	// frame does not.
	if n.Type == p.IDNode {
		sym, x, ok := scope.lookupNode(n)
		if !ok {
			throwAt(scope, n, ErrUndefinedVariable, "undefined variable '%s'", sym)
		}
		return x.Eval()
	}
	var y Expression
	y.setThunk(scope, n)
	return y.Eval()
}

// evalAttrPath evaluates the names of an attribute path, such as the
// `a."b".` of a binding or a selection. The path node is passed in rather
// than wrapped in an expression, since nothing needs it afterwards.
//
// A path of plain identifiers names the same attributes however often it is
// reached, so it is worked out once and kept against the node — which matters
// because a selection inside a loop is evaluated once per turn, and interning
// its components again each time was the largest single cost in the evaluator
// after allocation. A path with an interpolation in it has to be evaluated
// every time, and is not kept.
func (scope *Scope) evalAttrPath(path *p.Node) []Sym {
	e := scope.file.static.get(path.ID)
	if e.attrs != nil {
		return e.attrs
	}
	attrs := make([]Sym, len(path.Nodes))
	static := true
	for i, c := range path.Nodes {
		attrs[i] = scope.attrSym(c)
		static = static && c.Type == p.IDNode
	}
	if static {
		e.attrs = attrs
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
		if s.single() != nil {
			add(s.sym)
			continue
		}
		for _, sym := range s.binds().Keys() {
			add(sym)
		}
	}
	return names
}
