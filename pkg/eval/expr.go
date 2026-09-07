package eval

import (
	"fmt"

	p "github.com/orivej/go-nix/pkg/parser"
)

// blameKind labels why an expression is being evaluated, so that a backtrace
// frame can be described without building the string on the happy path.
type blameKind uint8

const (
	blameNone     blameKind = iota
	blameAttr               // "while evaluating the attribute 'x'"
	blameCall               // "while calling a function"
	blamePrimop             // "while calling the 'head' builtin"
	blameListElem           // "while evaluating a list element"
	blameInterp             // "while evaluating a string interpolation"
	blameSelect             // "while evaluating the attribute path"
	blameCond               // "while evaluating a condition"
	blameWith               // "while evaluating the set of a with expression"
	blameAssert             // "while evaluating an assertion"
)

func (b blameKind) describe(sym Sym) string {
	switch b {
	case blameAttr:
		return fmt.Sprintf("while evaluating the attribute '%s'", sym)
	case blameCall:
		return "while calling a function"
	case blamePrimop:
		return fmt.Sprintf("while calling the '%s' builtin", sym)
	case blameListElem:
		return "while evaluating a list element"
	case blameInterp:
		return "while evaluating a string interpolation"
	case blameSelect:
		return "while evaluating an attribute path"
	case blameCond:
		return "while evaluating a condition"
	case blameWith:
		return "while evaluating the set of a with expression"
	case blameAssert:
		return "while evaluating an assertion"
	}
	return ""
}

// Expression is a thunk: an unevaluated expression together with the scope it
// closes over, memoizing its value once forced.
//
// Exactly one of the following describes how the value is produced:
//   - Value is already known,
//   - Native computes it in Go (used by builtins),
//   - Node and Scope describe a piece of syntax to evaluate.
//
// resolve may replace the last case by Lower, an expression to evaluate in its
// stead (a `let` body, a function call result, a selected attribute); Eval then
// caches the result here as well, so a chain is only walked once.
type Expression struct {
	Value  NixValue
	Lower  *Expression
	Native func() NixValue
	Scope  *Scope
	Parser *p.Parser
	Node   *p.Node

	blame    blameKind
	blameSym Sym
	// blameNode is where the backtrace should point for this frame, when that
	// is not where the expression itself starts: a call frame is more useful
	// pointing at the function than at its body.
	blameNode *p.Node
	forcing   bool
}

// WithNode derives an unevaluated expression for a sibling node, in the same
// scope.
func (x *Expression) WithNode(n *p.Node) *Expression {
	return &Expression{Scope: x.Scope, Parser: x.Parser, Node: n}
}

// WithScoped derives an unevaluated expression for a node in a new scope.
func (x *Expression) WithScoped(n *p.Node, scope *Scope) *Expression {
	return &Expression{Scope: scope, Parser: x.Parser, Node: n}
}

// blaming labels x with what a backtrace should say about it. It returns x for
// chaining.
func (x *Expression) blaming(kind blameKind, sym Sym) *Expression {
	x.blame, x.blameSym = kind, sym
	return x
}

// blamingAt is blaming with an explicit position for the backtrace frame.
func (x *Expression) blamingAt(kind blameKind, sym Sym, n *p.Node) *Expression {
	x.blameNode = n
	return x.blaming(kind, sym)
}

// Pos reports where this expression starts, or nil when it has no syntax (a
// builtin thunk, or a value constructed by Go).
func (x *Expression) Pos() *p.LexPosition {
	if x.Parser == nil || x.Node == nil {
		return nil
	}
	return x.Parser.NodePos(x.Node)
}

func (x *Expression) frame() Frame {
	pos := x.Pos()
	if x.blameNode != nil && x.Parser != nil {
		pos = x.Parser.NodePos(x.blameNode)
	}
	var line string
	if pos != nil {
		line = x.Parser.SourceLine(pos)
	}
	return Frame{Pos: pos, Line: line, Desc: x.blame.describe(x.blameSym)}
}

// Eval forces the expression to a value, memoizing the result.
func (x *Expression) Eval() NixValue {
	if x.Value != nil {
		return x.Value
	}
	return x.force()
}

// maxCallDepth bounds how deeply expressions may nest at run time, so that a
// non-terminating definition is reported as an error instead of exhausting the
// Go stack. Nix imposes the same kind of limit.
const maxCallDepth = 20000

// evalStack holds the expressions currently being forced, innermost last. It
// is what an error is annotated from: throwf reads the position and the
// backtrace off it at the point of failure, so that unwinding stays a single
// panic rather than one re-panic per frame.
//
// Evaluation is single-goroutine, so a plain slice suffices.
var evalStack []*Expression

func (x *Expression) force() NixValue {
	if x.forcing {
		// The thunk is already being evaluated further up the stack, so its
		// value is defined in terms of itself.
		throwf(ErrInfiniteRecursion, "infinite recursion encountered")
	}
	x.forcing = true
	evalStack = append(evalStack, x)
	defer x.finish()
	if len(evalStack) > maxCallDepth {
		throwf(ErrEval, "stack overflow; possible infinite recursion")
	}

	switch {
	case x.Native != nil:
		x.Value = x.Native()
	case x.Node != nil:
		x.resolve()
	default:
		throwf(ErrEval, "expression has nothing to evaluate")
	}
	if x.Value == nil {
		// resolve delegated to another expression; evaluating it here (rather
		// than looping) keeps every thunk on the stack, so that cycles through
		// several bindings are detected and each contributes a trace frame.
		x.Value = x.Lower.Eval()
	}
	return x.Value
}

// finish leaves the expression, whether it produced a value or is being
// unwound past by a failure.
func (x *Expression) finish() {
	x.forcing = false
	evalStack = evalStack[:len(evalStack)-1]
}

// value wraps an already known value as an evaluated expression.
func value(v NixValue) *Expression {
	return &Expression{Value: v}
}

// thunk wraps a Go computation as an unevaluated expression.
func thunk(f func() NixValue) *Expression {
	return &Expression{Native: f}
}
