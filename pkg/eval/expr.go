package eval

import (
	"fmt"

	p "github.com/aleksanaa/go-nix/pkg/parser"
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
//   - Node describes a piece of syntax to evaluate, in Scope.
//
// resolve may replace the last case by Lower, an expression to evaluate in its
// stead (a `let` body, a function call result, a selected attribute); Eval then
// caches the result here as well, so a chain is only walked once.
//
// The field order is chosen to keep the struct at 64 bytes, since evaluating
// anything substantial allocates millions of these.
type Expression struct {
	Value  NixValue
	Lower  *Expression
	Native func() NixValue
	Scope  *Scope
	Node   *p.Node

	// blameNode is where the backtrace should point for this frame, when that
	// is not where the expression itself starts: a call frame is more useful
	// pointing at the function than at its body.
	blameNode *p.Node
	blameSym  Sym
	blame     blameKind
	forcing   bool
}

// exprSlabSize is how many expressions are allocated at a time, or 1 to
// allocate them one by one.
//
// Handing thunks out from a block trades one allocation per thunk for one per
// block, the same trick the parser uses for nodes, and it does measurably
// speed evaluation up. It is off by default because a block cannot be freed
// until every thunk in it is unreachable, and thunks that survive evaluation
// are interleaved with ones that do not. On the Hanoi example, blocks of 256
// bought 14% of the time for 2.4 times the peak memory, which is the wrong
// trade for an evaluator meant to be pointed at large expressions. Workloads
// whose thunks nearly all die young — see examples/hanoi-calls.nix — get the
// speed without the memory, so this is worth revisiting per use.
const exprSlabSize = 1

// exprSlab is the block currently being handed out. Evaluation is
// single-goroutine, like the symbol table and the evaluation stack.
var exprSlab []Expression

// newExpr returns a zeroed expression. The block branch folds away when
// blocks are disabled.
func newExpr() *Expression {
	if exprSlabSize <= 1 {
		return new(Expression)
	}
	if len(exprSlab) == 0 {
		exprSlab = make([]Expression, exprSlabSize)
	}
	x := &exprSlab[0]
	exprSlab = exprSlab[1:]
	return x
}

// parser is the parser the expression's node belongs to. It is reached through
// the scope rather than stored per expression, of which there are far more.
func (x *Expression) parser() *p.Parser {
	if x.Scope == nil {
		return nil
	}
	return x.Scope.parser()
}

// WithNode derives an unevaluated expression for a sibling node, in the same
// scope.
func (x *Expression) WithNode(n *p.Node) *Expression {
	y := newExpr()
	y.Scope, y.Node = x.Scope, n
	return y
}

// WithScoped derives an unevaluated expression for a node in a new scope.
func (x *Expression) WithScoped(n *p.Node, scope *Scope) *Expression {
	return newScoped(scope, n)
}

// newScoped is an unevaluated expression for a node in a scope.
func newScoped(scope *Scope, n *p.Node) *Expression {
	y := newExpr()
	y.Scope, y.Node = scope, n
	return y
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
	pr := x.parser()
	if pr == nil || x.Node == nil {
		return nil
	}
	return pr.NodePos(x.Node)
}

// parser is the file the frame's node was parsed from, or nil for a frame with
// no syntax behind it, such as a builtin.
func (f evalFrame) parser() *p.Parser {
	return f.scope.parser()
}

// pos reports where the frame's expression starts, or nil when it has no
// syntax (a builtin thunk, or a value constructed by Go).
func (f evalFrame) pos() *p.LexPosition {
	pr := f.parser()
	if pr == nil || f.node == nil {
		return nil
	}
	return pr.NodePos(f.node)
}

// traceFrame renders the frame for a backtrace, pointing at blameNode when the
// expression itself is not the most useful place to point.
func (f evalFrame) traceFrame() Frame {
	pr := f.parser()
	pos := f.pos()
	if f.blameNode != nil && pr != nil {
		pos = pr.NodePos(f.blameNode)
	}
	var line string
	if pos != nil {
		line = pr.SourceLine(pos)
	}
	return Frame{Pos: pos, Line: line, Desc: f.blame.describe(f.blameSym)}
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

// evalFrame is what the evaluation stack records about one expression being
// forced. It is a copy of the few fields an error report needs rather than a
// pointer, so that an expression evaluated only for its value never has to
// outlive the Go stack frame that forced it.
type evalFrame struct {
	scope     *Scope
	node      *p.Node
	blameNode *p.Node
	blameSym  Sym
	blame     blameKind
}

// evalStack holds the expressions currently being forced, innermost last. It
// is what an error is annotated from: throwf reads the position and the
// backtrace off it at the point of failure, so that unwinding stays a single
// panic rather than one re-panic per frame.
//
// Evaluation is single-goroutine, so a plain slice suffices.
var evalStack []evalFrame

func (x *Expression) force() NixValue {
	if x.forcing {
		// The thunk is already being evaluated further up the stack, so its
		// value is defined in terms of itself.
		throwf(ErrInfiniteRecursion, "infinite recursion encountered")
	}
	x.forcing = true
	evalStack = append(evalStack, evalFrame{
		scope: x.Scope, node: x.Node,
		blameNode: x.blameNode, blameSym: x.blameSym, blame: x.blame,
	})
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
	// The thunk is now nothing but its value, so drop what only producing it
	// needed. This releases the scope chain and the expressions underneath,
	// which otherwise stay reachable for as long as the value does. Failure
	// leaves everything in place: the expression is still unevaluated, and the
	// error being raised is describing it from the evaluation stack.
	x.Lower, x.Native, x.Scope, x.Node, x.blameNode = nil, nil, nil, nil, nil
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
	x := newExpr()
	x.Value = v
	return x
}

// thunk wraps a Go computation as an unevaluated expression.
func thunk(f func() NixValue) *Expression {
	x := newExpr()
	x.Native = f
	return x
}

// evalNode evaluates a child node in this expression's scope.
//
// It is for operands that are consumed immediately — the two sides of an
// operator, a condition, the function of an application — where nothing keeps
// the thunk afterwards. Because the evaluation stack stores copies rather than
// pointers, the expression can stay on the Go stack, which is what makes an
// arithmetic-heavy expression allocate nothing per operand. Where a value does
// escape, the compiler falls back to allocating it, so this stays correct
// wherever it is used.
func (x *Expression) evalNode(n *p.Node) NixValue { return x.Scope.evalNode(n) }

// evalNodeAs is evalNode for an operand that a backtrace should name.
func (x *Expression) evalNodeAs(n *p.Node, kind blameKind, sym Sym) NixValue {
	y := Expression{Scope: x.Scope, Node: n, blame: kind, blameSym: sym}
	return y.Eval()
}
