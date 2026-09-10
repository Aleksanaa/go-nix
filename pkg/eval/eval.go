// Package eval implements a lazy evaluator for the Nix expression language.
//
// Evaluation is thunk based: every expression is an [Expression], forced at
// most once and memoized. Failures are raised as panics carrying an
// [EvalError] and are annotated with a source position and a backtrace as they
// unwind; Eval and EvalString at the top of this file turn them back into
// ordinary Go errors.
package eval

import (
	"strconv"
	"strings"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// Eval evaluates a parsed expression to a value.
//
// The result is only evaluated as far as its outermost value: forcing what it
// contains, as Print does, can fail in turn, so use Print rather than calling
// the method directly.
func Eval(pr *p.Parser) (NixValue, error) {
	return EvalIn(DefaultScope, pr)
}

// EvalIn evaluates a parsed expression in a scope other than the default one.
// It is how the REPL evaluates an entry, so that the entry sees the names the
// session has bound so far.
func EvalIn(scope *Scope, pr *p.Parser) (NixValue, error) {
	return catching(func() NixValue { return Delay(scope, pr).Eval() })
}

// Delay returns a parsed expression as an unforced thunk in a scope.
//
// The REPL binds names to these rather than to values, which is what makes
// `x = <something that fails>` legal until x is used, just as a `let` is.
func Delay(scope *Scope, pr *p.Parser) *Expression {
	x := newExpr()
	x.Scope, x.Node = scope.ForFile(pr), pr.Result
	return x
}

// Print renders a value, forcing it down to the given depth. A negative depth
// forces it completely.
func Print(val NixValue, depth int) (string, error) {
	return catching(func() string { return val.Print(depth) })
}

// catching runs f, turning an evaluation failure into an error. Any other
// panic is a bug in the evaluator and keeps unwinding.
func catching[T any](f func() T) (result T, err error) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		e := asEvalError(v)
		if e == nil {
			panic(v)
		}
		var zero T
		result, err = zero, e
	}()
	return f(), nil
}

// EvalString parses and evaluates a Nix expression given as text.
func EvalString(s string) (NixValue, error) {
	pr, err := p.ParseString(s)
	if err != nil {
		return nil, err
	}
	return Eval(pr)
}

// resolve evaluates one syntax node. It either sets Value, points the
// expression at another node to continue with in place, or returns an
// expression to evaluate in its stead — one that already exists, and that
// something else may be holding.
func (x *Expression) resolve() *Expression {
	n := x.Node
	switch nt := n.Type; nt {
	default:
		throwf(ErrEval, "unsupported expression: %v", nt)

	// A literal is the same value however often it is evaluated, so each of
	// these is worked out once and kept against the node.
	case p.URINode:
		x.Value = x.Scope.literal(n, func(s string) NixValue { return String(s) })

	case p.PathNode:
		// TODO: resolve relative to the file being evaluated, and <lookup>
		// paths through NIX_PATH.
		x.Value = x.Scope.literal(n, func(s string) NixValue {
			return &NixPath{Root: "/", Path: s}
		})

	case p.FloatNode:
		x.Value = x.Scope.literal(n, func(s string) NixValue {
			val, err := strconv.ParseFloat(s, 64)
			if err != nil {
				throwf(ErrSyntax, "invalid float %q", s)
			}
			return NixFloat(val)
		})

	case p.IntNode:
		x.Value = x.Scope.literal(n, func(s string) NixValue {
			val, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				throwf(ErrSyntax, "invalid integer %q", s)
			}
			return NixInt(val)
		})

	case p.StringNode, p.IStringNode:
		x.Value = x.evalString()

	case p.IDNode:
		sym := x.Scope.name(n)
		y, ok := x.Scope.Lookup(sym)
		if !ok {
			throwf(ErrUndefinedVariable, "undefined variable '%s'", sym)
		}
		return y

	case p.ParensNode:
		x.continueAt(n.Nodes[0], x.Scope)

	case p.ListNode:
		list := make(NixList, len(n.Nodes))
		for i, c := range n.Nodes {
			list[i] = x.WithNode(c).blaming(blameListElem)
		}
		x.Value = list

	case p.SetNode, p.RecSetNode, p.LetNode:
		x.evalBinds(nt)

	case p.SelectNode, p.SelectOrNode:
		return x.evalSelect(nt)

	case p.WithNode:
		attrs := assertSet(x.evalNodeAs(n.Nodes[0], blameWith))
		x.continueAt(n.Nodes[1], x.Scope.Subscope(attrs, true))

	case p.IfNode:
		cond := assertBool(x.evalNodeAs(n.Nodes[0], blameCond))
		if cond {
			x.continueAt(n.Nodes[1], x.Scope)
		} else {
			x.continueAt(n.Nodes[2], x.Scope)
		}

	case p.AssertNode:
		if !assertBool(x.evalNodeAs(n.Nodes[0], blameAssert)) {
			throwf(ErrAssertion, "assertion '%s' failed", x.parser().NodeString(n.Nodes[0]))
		}
		x.continueAt(n.Nodes[1], x.Scope)

	case p.FunctionNode:
		x.Value = x.evalFunction()

	case p.ApplyNode:
		// `f a b` parses as `(f a) b` and is applied in one step, so that the
		// function value in between the two arguments is never built. A longer
		// chain is entered two arguments at a time.
		if inner := n.Nodes[0]; inner.Type == p.ApplyNode {
			fn := assertLambda(x.evalNode(inner.Nodes[0]))
			a, b := x.thunkFor(inner.Nodes[1]), x.thunkFor(n.Nodes[1])
			if applyIn2(x, fn, a, b) {
				break
			}
			return apply2(fn, a, b)
		}
		fn := assertLambda(x.evalNode(n.Nodes[0]))
		arg := x.thunkFor(n.Nodes[1])
		if !applyIn(x, fn, arg) {
			return fn.Apply(arg)
		}

	case p.OpNegateNode, p.OpNotNode, p.OpQuestionNode:
		x.Value = x.evalUnaryOp(nt)

	case p.OpAddNode, p.OpReduceNode, p.OpMultiplyNode, p.OpDivideNode,
		p.OpGreaterNode, p.OpLessNode, p.OpGeqNode, p.OpLeqNode,
		p.OpConcatNode, p.OpUpdateNode, p.OpAndNode, p.OpOrNode, p.OpImplNode,
		p.OpEqNode, p.OpNeqNode:
		x.Value = x.evalBinaryOp(nt)
	}
	return nil
}

// evalString evaluates a quoted or an indented string literal. One with no
// interpolation in it is a literal like any other, and is kept against the
// node rather than rebuilt on every evaluation.
func (x *Expression) evalString() NixValue {
	entry := x.Scope.file.static.get(x.Node.ID)
	if entry.val != nil {
		return entry.val
	}
	interpolated := false
	parts := make([]stringPart, 0, len(x.Node.Nodes))
	indented := x.Node.Type == p.IStringNode
	for _, c := range x.Node.Nodes {
		switch c.Type {
		default:
			throwf(ErrEval, "unsupported string part: %v", c.Type)
		case p.TextNode:
			parts = append(parts, stringPart{text: x.parser().TokenString(c.Tokens[0])})
		case p.InterpNode:
			// Interpolations are evaluated in source order, as Nix does.
			interpolated = true
			part := x.evalNodeAs(c.Nodes[0], blameInterp)
			parts = append(parts, stringPart{interp: CoerceToString(part)})
		}
	}
	if indented {
		parts = stripIndentation(parts)
	}

	result := &NixString{}
	var b strings.Builder
	for _, part := range parts {
		if part.interp != nil {
			b.WriteString(part.interp.Content)
			result.absorb(part.interp)
			continue
		}
		if indented {
			b.WriteString(unescapeIndented(part.text))
		} else {
			b.WriteString(unescapeQuoted(part.text))
		}
	}
	result.Content = b.String()
	if !interpolated {
		entry.val = result
	}
	return result
}

// evalBinds evaluates a set, a recursive set, or the bindings of a `let`.
func (x *Expression) evalBinds(nt p.NodeType) {
	n := x.Node
	bindNodes := n.Nodes
	if nt == p.LetNode {
		bindNodes = n.Nodes[0].Nodes
	}
	// Inherited bindings make the set larger than this estimate.
	set := make(NixSet, len(bindNodes))
	scope := x.Scope
	if nt == p.RecSetNode || nt == p.LetNode {
		// A recursive set and a `let` are in scope of their own bindings.
		scope = scope.Subscope(set, false)
	}
	for _, c := range bindNodes {
		switch c.Type {
		default:
			throwf(ErrEval, "unsupported binding: %v", c.Type)

		case p.BindNode:
			attrpath := scope.evalAttrPath(c.Nodes[0])
			y := x.WithScoped(c.Nodes[1], scope)
			set.Bind(attrpath, y.blamingAttr(attrpath[len(attrpath)-1]))

		case p.InheritNode:
			// `inherit a;` is `a = a;` evaluated in the enclosing scope.
			for _, id := range c.Nodes[0].Nodes {
				y := x.WithNode(id)
				sym := x.Scope.attrSym(id)
				set.Bind1(sym, y.blamingAttr(sym))
			}

		case p.InheritFromNode:
			// `inherit (e) a;` is `a = (e).a;`; e itself stays lazy, and is
			// shared by every name inherited from it.
			from := x.WithScoped(c.Nodes[0], scope)
			for _, id := range c.Nodes[1].Nodes {
				sym := x.Scope.attrSym(id)
				set.Bind1(sym, from.selectAttr(sym).blamingAttr(sym))
			}
		}
	}
	if nt == p.LetNode {
		x.continueAt(n.Nodes[1], scope)
	} else {
		x.Value = set
	}
}

// evalSelect evaluates `e.a.b` and `e.a.b or fallback`, returning the
// expression the path selects.
func (x *Expression) evalSelect(nt p.NodeType) *Expression {
	n := x.Node
	attrpath := x.Scope.evalAttrPath(n.Nodes[1])
	var or *Expression
	if nt == p.SelectOrNode {
		or = x.WithNode(n.Nodes[2])
	}
	// Only the leading expression is labelled: the attributes selected along
	// the way are shared with the set that holds them, and already carry their
	// own label.
	expr := x.WithNode(n.Nodes[0]).blaming(blameSelect)
	for _, sym := range attrpath {
		// As in Nix, `or` also covers selecting from a non-set.
		set, ok := expr.Eval().(NixSet)
		if ok {
			if y, found := set[sym]; found {
				expr = y
				continue
			}
		}
		if or != nil {
			expr = or
			break
		}
		if !ok {
			throwf(ErrType, "value is %s while a set was expected", anTypeName(expr.Value))
		}
		throwf(ErrMissingAttribute, "attribute '%s' missing", sym)
	}
	return expr
}

// evalFunction builds a closure from a function node.
//
// Everything but the scope is decided by the syntax, so it is worked out once
// per node and shared: a closure is then two words. That matters because a
// function written inside another one — every curried definition — is created
// afresh on each call, and rebuilding the formal-argument map with it was one
// of the largest sources of allocation in the evaluator.
func (x *Expression) evalFunction() NixValue {
	return &NixExprLambda{lambdaInfo: x.Scope.lambdaInfo(x.Node), Scope: x.Scope}
}

// lambdaInfo describes a function node: the names it binds and where its body
// is. The grammar hands us the body last, preceded by an identifier
// (`a: …` or `…@a: …`) and/or a formal argument set (`{ a, b ? 1, ... }: …`).
func (scope *Scope) lambdaInfo(n *p.Node) *lambdaInfo {
	e := scope.file.static.get(n.ID)
	if e.lambda != nil {
		return e.lambda
	}
	fn := &lambdaInfo{Node: n, Body: n.Nodes[len(n.Nodes)-1]}
	// A call's frame points at the function rather than at its body, so the
	// body records which function it belongs to.
	scope.file.static.get(fn.Body.ID).owner = n
	for _, c := range n.Nodes[:len(n.Nodes)-1] {
		switch c.Type {
		case p.IDNode:
			fn.Arg, fn.HasArg = scope.name(c), true
		case p.ArgSetNode:
			fn.HasFormal = true
			fn.Formal = make(map[Sym]*p.Node, len(c.Nodes))
			fn.FormalOrder = make([]Sym, 0, len(c.Nodes))
			for _, arg := range c.Nodes {
				if len(arg.Nodes) == 0 {
					fn.HasEllipsis = true // `...`
					continue
				}
				sym := scope.name(arg.Nodes[0])
				var def *p.Node // `a ? default`
				if len(arg.Nodes) == 2 {
					def = arg.Nodes[1]
				}
				if _, dup := fn.Formal[sym]; dup {
					throwf(ErrEval, "duplicate formal function argument '%s'", sym)
				}
				fn.Formal[sym] = def
				fn.FormalOrder = append(fn.FormalOrder, sym)
			}
		default:
			throwf(ErrEval, "unsupported function part: %v", c.Type)
		}
	}
	if fn.HasArg && fn.HasFormal {
		if _, dup := fn.Formal[fn.Arg]; dup {
			throwf(ErrEval, "duplicate formal function argument '%s'", fn.Arg)
		}
	}
	// Only a function the evaluator accepted is kept, so that one it rejects
	// reports itself however often it is evaluated.
	e.lambda = fn
	return fn
}

// selectAttr returns the attribute sym of the set this expression evaluates
// to, without forcing it yet.
func (x *Expression) selectAttr(sym Sym) *Expression {
	return thunk(func() NixValue {
		set := assertSet(x.Eval())
		y, ok := set[sym]
		if !ok {
			throwf(ErrMissingAttribute, "attribute '%s' missing", sym)
		}
		return y.Eval()
	})
}
