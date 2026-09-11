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
// but a cheaper one, since a scope chain dies together. 1024 measured fastest
// on the call-heavy workloads; a scope chain rarely lives past one block.
const scopeSlabSize = 1024

// Subscope nests a scope binding a set of names.
func (scope *Scope) subscope(w *worker, binds NixSet, lowPrio bool) *Scope {
	s := w.newScope()
	*s = Scope{bound: unsafe.Pointer(binds), LowPrio: lowPrio, Parent: scope, file: scope.file}
	return s
}

// Subscope1 nests a scope binding a single name.
func (scope *Scope) Subscope1(w *worker, sym Sym, x *Expression) *Scope {
	s := w.newScope()
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

// withScope nests a scope that provides a with-set, without forcing the set.
// The set is only forced when a name is actually looked up in it, so that a
// fixpoint whose body is `with self; …` does not recurse.
func (scope *Scope) withScope(w *worker, x *Expression) *Scope {
	s := w.newScope()
	*s = Scope{bound: unsafe.Pointer(x), LowPrio: true, Parent: scope, file: scope.file}
	return s
}

// withSet is the set a with scope's names come from, forced on first use.
func (s *Scope) withSet(w *worker) NixSet {
	return assertSet(w, (*Expression)(s.bound).Eval(w))
}

// lookupFrom finds sym, searching lexical bindings first and `with` bindings
// only afterwards. It also reports where a lexical binding was found — how many
// scopes had to be skipped, and which slot of that scope holds it. A `with`
// reports neither: what its set holds is not decided until it is evaluated,
// and a nearer `with` shadows a farther one, so there is nothing about it
// worth remembering.
//
// The two are searched in two passes, not one: a lexical binding anywhere in
// the chain beats every `with`, and a `with` set must not be forced until that
// is known. Forcing it during the walk would recurse on a fixpoint whose body
// is `with self; …`, where a name resolves lexically but only after the walk
// has gone past the `with` scope.
func (scope *Scope) lookupFrom(w *worker, sym Sym) (*Expression, int32, int32, bool) {
	if x, hops, slot, ok := scope.lookupLexicalFrom(sym); ok {
		return x, hops, slot, true
	}
	// Nothing lexical: the nearest `with` set that has the name wins.
	for s := scope; s != nil; s = s.Parent {
		if s.LowPrio {
			if y, found := s.withSet(w).Get(sym); found {
				return y, -1, -1, true
			}
		}
	}
	return nil, -1, -1, false
}

// lookupLexicalFrom is the half of the search that the scopes answer outright,
// without evaluating anything: a `with` set would have to be forced to say
// whether it has a name, and this never forces one.
func (scope *Scope) lookupLexicalFrom(sym Sym) (x *Expression, hops, slot int32, ok bool) {
	for s := scope; s != nil; s, hops = s.Parent, hops+1 {
		if s.LowPrio {
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
	return nil, -1, -1, false
}

// lookupNode finds what an identifier node refers to.
//
// Which scope holds a name is decided by the syntax: the same identifier node,
// evaluated again, is reached through a chain of scopes of the same shape. So
// the number of scopes to skip is remembered and the next evaluation jumps
// straight to it, instead of probing a set at every level on the way. Nix
// settles this once and for all at parse time; this arrives at the same place
// without a pass of its own, and checks the name it lands on, so that a chain
// of a shape it did not expect costs a search rather than a wrong answer.
//
// What is remembered belongs to the worker rather than to the syntax: a slot
// is a position in a set built at run time, so it is a fact about this
// evaluation. See lookupMemo.
func (scope *Scope) lookupNode(w *worker, n *p.Node) (Sym, *Expression, bool) {
	sym := scope.file.static.get(n.ID).sym
	m := w.remember(scope.file, n.ID)
	if m.hops < 0 {
		// Nothing in the chain binds this name lexically, which the syntax
		// decides once and for all, so only a `with` can have it and the
		// nearest one wins. Nix marks such a name the same way.
		for s := scope; s != nil; s = s.Parent {
			if s.LowPrio {
				if x, ok := s.withSet(w).Get(sym); ok {
					return sym, x, true
				}
			}
		}
		return sym, nil, false
	}
	if m.hops > 0 {
		s := scope
		for i := m.hops - 1; i > 0 && s != nil; i-- {
			s = s.Parent
		}
		if s != nil {
			if s.sym != 0 {
				if s.sym == sym {
					return sym, (*Expression)(s.bound), true
				}
			} else if x, ok := (*AttrSet)(s.bound).at(m.slot-1, sym); ok {
				// The name was in this slot last time and still is, which
				// is the whole lookup: two loads and a compare.
				return sym, x, true
			}
		}
	}
	x, hops, slot, ok := scope.lookupFrom(w, sym)
	if hops >= 0 {
		m.hops, m.slot = hops+1, slot+1
	} else {
		// Either a `with` provided it or nothing did; both mean there is no
		// lexical binding to find next time.
		m.hops = -1
	}
	return sym, x, ok
}

// evalNode evaluates a node in this scope, with no surrounding expression to
// take the scope from.
func (scope *Scope) evalNode(w *worker, n *p.Node) NixValue {
	// Reading a name is the value of the thunk it is bound to. An expression
	// of its own would mean forcing twice — once for the read, once for the
	// binding — and a backtrace frame that says nothing the binding's own
	// frame does not.
	if n.Type == p.IDNode {
		sym, x, ok := scope.lookupNode(w, n)
		if !ok {
			w.throwAt(scope, n, ErrUndefinedVariable, "undefined variable '%s'", sym)
		}
		return x.Eval(w)
	}
	// A literal is already a value: it needs no thunk, no evaluation frame
	// and no forcing, only the number read out of the node's cache.
	if val, ok := scope.literalValue(w, n); ok {
		return val
	}
	var y Expression
	y.setThunk(scope, n)
	return y.Eval(w)
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
func (scope *Scope) evalAttrPath(w *worker, path *p.Node) []Sym {
	e := scope.file.static.get(path.ID)
	if e.attrs != nil {
		return e.attrs
	}
	// A path with an interpolation in it names something different every
	// time, so it is worked out here and kept by nobody.
	attrs := make([]Sym, len(path.Nodes))
	for i, c := range path.Nodes {
		attrs[i] = scope.attrSym(w, c)
	}
	return attrs
}

// attrPathNull reports whether any dynamic component of an attribute path
// evaluates to null, which is what skips a binding in Nix: `{ ${null} = 1; }`
// binds nothing.
func (scope *Scope) attrPathNull(w *worker, path *p.Node) bool {
	for _, c := range path.Nodes {
		if c.Type == p.InterpNode && scope.evalNode(w, c.Nodes[0]).Kind() == KindNull {
			return true
		}
	}
	return false
}

// attrSym evaluates one component of an attribute path to its interned name.
// A plain identifier is interned once and kept against the node; a computed
// one has to be evaluated every time.
func (scope *Scope) attrSym(w *worker, n *p.Node) Sym {
	switch n.Type {
	case p.IDNode:
		return scope.name(n)
	case p.StringNode, p.IStringNode:
		return CoerceToString(w, scope.evalNode(w, n)).intern()
	case p.InterpNode:
		return CoerceToString(w, scope.evalNode(w, n.Nodes[0])).intern()
	default:
		w.throwf(ErrEval, "unsupported attribute name: %v", n.Type)
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

// Subscope nests a scope binding a set of names, for a caller outside the
// evaluator — the REPL, binding what a session has named so far. Evaluation
// inside the package carries the worker it is running on; out here there is
// only the one.
func (scope *Scope) Subscope(binds NixSet, lowPrio bool) *Scope {
	return scope.subscope(mainWorker, binds, lowPrio)
}

// lookupBorrow finds the binding an identifier names, for a place that would
// borrow it rather than make a thunk of its own — a list element, a call
// argument, the value of an attribute.
//
// It answers out of the scopes alone. A name they do not bind could still come
// from a `with`, and finding out means forcing that `with` set, which would
// evaluate something the expression may never ask for; so a name like that is
// not borrowed and stays a thunk. Nix stops at the same line, in lookupVar's
// noEval flag, whose comment says the same: it gives up the sharing for `with`
// rather than pay to keep it.
//
// A group still being built is the other place it gives up. The names bound
// above this point are in the set already and can be borrowed; a name the
// group binds further down is not there yet, and must not be looked for
// further out, where it would find the binding this one shadows. Nix meets the
// same moment as a slot it has not filled in — see ExprVar::maybeThunk, "The
// value might not be initialised in the environment yet" — and does the same
// thing, which is to make an ordinary thunk and let the force resolve it.
func (scope *Scope) lookupBorrow(w *worker, n *p.Node) (*Expression, bool) {
	if m := w.remember(scope.file, n.ID); m.hops < 0 {
		// The syntax settled it: nothing in the chain binds this name, so only
		// a `with` can have it and there is nothing to borrow.
		return nil, false
	}
	sym := scope.file.static.get(n.ID).sym
	for s := scope; s != nil; s = s.Parent {
		if s.LowPrio {
			continue // a `with`: it would have to be forced to answer
		}
		if s.sym != 0 {
			if s.sym == sym {
				return (*Expression)(s.bound), true
			}
			continue
		}
		set := (*AttrSet)(s.bound)
		if set == nil {
			continue
		}
		if !set.sorted {
			return set.bound(sym)
		}
		if y, _, found := set.getSlot(sym); found {
			return y, true
		}
	}
	return nil, false
}
