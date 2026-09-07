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

	p "github.com/orivej/go-nix/pkg/parser"
)

// Eval evaluates a parsed expression to a value.
//
// The result is only evaluated as far as its outermost value: forcing what it
// contains, as Print does, can fail in turn, so use Print rather than calling
// the method directly.
func Eval(pr *p.Parser) (NixValue, error) {
	return catching(func() NixValue {
		x := &Expression{Parser: pr, Node: pr.Result, Scope: DefaultScope}
		return x.Eval()
	})
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

func (x *Expression) tokenString(i int) string {
	return x.Parser.TokenString(x.Node.Tokens[i])
}

// resolve evaluates one syntax node, setting either Value (the result) or
// Lower (an expression to evaluate in its place).
func (x *Expression) resolve() {
	n := x.Node
	switch nt := n.Type; nt {
	default:
		throwf(ErrEval, "unsupported expression: %v", nt)

	case p.URINode:
		x.Value = &NixString{Content: x.tokenString(0)}

	case p.PathNode:
		// TODO: resolve relative to the file being evaluated, and <lookup>
		// paths through NIX_PATH.
		x.Value = &NixPath{Root: "/", Path: x.tokenString(0)}

	case p.FloatNode:
		val, err := strconv.ParseFloat(x.tokenString(0), 64)
		if err != nil {
			throwf(ErrSyntax, "invalid float %q", x.tokenString(0))
		}
		x.Value = NixFloat(val)

	case p.IntNode:
		val, err := strconv.ParseInt(x.tokenString(0), 10, 64)
		if err != nil {
			throwf(ErrSyntax, "invalid integer %q", x.tokenString(0))
		}
		x.Value = NixInt(val)

	case p.StringNode, p.IStringNode:
		x.Value = x.evalString()

	case p.IDNode:
		sym := Intern(x.tokenString(0))
		y, ok := x.Scope.Lookup(sym)
		if !ok {
			throwf(ErrUndefinedVariable, "undefined variable '%s'", sym)
		}
		x.Lower = y

	case p.ParensNode:
		x.Lower = x.WithNode(n.Nodes[0])

	case p.ListNode:
		list := make(NixList, len(n.Nodes))
		for i, c := range n.Nodes {
			list[i] = x.WithNode(c).blaming(blameListElem, 0)
		}
		x.Value = list

	case p.SetNode, p.RecSetNode, p.LetNode:
		x.evalBinds(nt)

	case p.SelectNode, p.SelectOrNode:
		x.evalSelect(nt)

	case p.WithNode:
		attrs := x.WithNode(n.Nodes[0]).blaming(blameWith, 0)
		x.Lower = x.WithScoped(n.Nodes[1], x.Scope.Subscope(assertSet(attrs.Eval()), true))

	case p.IfNode:
		cond := assertBool(x.WithNode(n.Nodes[0]).blaming(blameCond, 0).Eval())
		if cond {
			x.Lower = x.WithNode(n.Nodes[1])
		} else {
			x.Lower = x.WithNode(n.Nodes[2])
		}

	case p.AssertNode:
		if !assertBool(x.WithNode(n.Nodes[0]).blaming(blameAssert, 0).Eval()) {
			throwf(ErrAssertion, "assertion '%s' failed", x.Parser.NodeString(n.Nodes[0]))
		}
		x.Lower = x.WithNode(n.Nodes[1])

	case p.FunctionNode:
		x.Value = x.evalFunction()

	case p.ApplyNode:
		fn := assertLambda(x.WithNode(n.Nodes[0]).Eval())
		x.Lower = fn.Apply(x.WithNode(n.Nodes[1]))

	case p.OpNegateNode, p.OpNotNode, p.OpQuestionNode:
		x.Value = x.evalUnaryOp(nt)

	case p.OpAddNode, p.OpReduceNode, p.OpMultiplyNode, p.OpDivideNode,
		p.OpGreaterNode, p.OpLessNode, p.OpGeqNode, p.OpLeqNode,
		p.OpConcatNode, p.OpUpdateNode, p.OpAndNode, p.OpOrNode, p.OpImplNode,
		p.OpEqNode, p.OpNeqNode:
		x.Value = x.evalBinaryOp(nt)
	}
}

// evalString evaluates a quoted or an indented string literal.
func (x *Expression) evalString() NixValue {
	parts := make([]stringPart, 0, len(x.Node.Nodes))
	indented := x.Node.Type == p.IStringNode
	for _, c := range x.Node.Nodes {
		switch c.Type {
		default:
			throwf(ErrEval, "unsupported string part: %v", c.Type)
		case p.TextNode:
			parts = append(parts, stringPart{text: x.Parser.TokenString(c.Tokens[0])})
		case p.InterpNode:
			// Interpolations are evaluated in source order, as Nix does.
			y := x.WithNode(c.Nodes[0]).blaming(blameInterp, 0)
			parts = append(parts, stringPart{interp: CoerceToString(y.Eval())})
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
			attrpath := x.WithScoped(c.Nodes[0], scope).evalAttrPath()
			y := x.WithScoped(c.Nodes[1], scope)
			set.Bind(attrpath, y.blaming(blameAttr, attrpath[len(attrpath)-1]))

		case p.InheritNode:
			// `inherit a;` is `a = a;` evaluated in the enclosing scope.
			for _, id := range c.Nodes[0].Nodes {
				y := x.WithNode(id)
				sym := Intern(y.attrName())
				set.Bind1(sym, y.blaming(blameAttr, sym))
			}

		case p.InheritFromNode:
			// `inherit (e) a;` is `a = (e).a;`; e itself stays lazy, and is
			// shared by every name inherited from it.
			from := x.WithScoped(c.Nodes[0], scope)
			for _, id := range c.Nodes[1].Nodes {
				sym := Intern(x.WithNode(id).attrName())
				set.Bind1(sym, from.selectAttr(sym).blaming(blameAttr, sym))
			}
		}
	}
	if nt == p.LetNode {
		x.Lower = x.WithScoped(n.Nodes[1], scope)
	} else {
		x.Value = set
	}
}

// evalSelect evaluates `e.a.b` and `e.a.b or fallback`.
func (x *Expression) evalSelect(nt p.NodeType) {
	n := x.Node
	attrpath := x.WithNode(n.Nodes[1]).evalAttrPath()
	var or *Expression
	if nt == p.SelectOrNode {
		or = x.WithNode(n.Nodes[2])
	}
	// Only the leading expression is labelled: the attributes selected along
	// the way are shared with the set that holds them, and already carry their
	// own label.
	expr := x.WithNode(n.Nodes[0]).blaming(blameSelect, 0)
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
	x.Lower = expr
}

// evalFunction builds a closure from a function node. The grammar hands us the
// body last, preceded by an identifier (`a: …` or `…@a: …`) and/or a formal
// argument set (`{ a, b ? 1, ... }: …`).
func (x *Expression) evalFunction() NixValue {
	n := x.Node
	fn := &NixExprLambda{Expression: x, Body: n.Nodes[len(n.Nodes)-1]}
	for _, c := range n.Nodes[:len(n.Nodes)-1] {
		switch c.Type {
		case p.IDNode:
			fn.Arg, fn.HasArg = Intern(x.WithNode(c).tokenString(0)), true
		case p.ArgSetNode:
			fn.HasFormal = true
			fn.Formal = make(map[Sym]*p.Node, len(c.Nodes))
			fn.FormalOrder = make([]Sym, 0, len(c.Nodes))
			for _, arg := range c.Nodes {
				if len(arg.Nodes) == 0 {
					fn.HasEllipsis = true // `...`
					continue
				}
				sym := Intern(x.WithNode(arg.Nodes[0]).tokenString(0))
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

// evalAttrPath evaluates the names of an attribute path, such as the
// `a."b".${c}` of a binding or a selection.
func (x *Expression) evalAttrPath() []Sym {
	attrs := make([]Sym, len(x.Node.Nodes))
	for i, c := range x.Node.Nodes {
		attrs[i] = Intern(x.WithNode(c).attrName())
	}
	return attrs
}

// attrName evaluates one component of an attribute path to its name.
func (x *Expression) attrName() string {
	switch x.Node.Type {
	case p.IDNode:
		return x.tokenString(0)
	case p.StringNode, p.IStringNode:
		return CoerceToString(x.Eval()).Content
	case p.InterpNode:
		y := x.WithNode(x.Node.Nodes[0]).blaming(blameInterp, 0)
		return CoerceToString(y.Eval()).Content
	default:
		throwf(ErrEval, "unsupported attribute name: %v", x.Node.Type)
		return ""
	}
}
