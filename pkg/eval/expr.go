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
// Resolving the last case may point the expression at another node to carry on
// with in place, or hand back an expression to evaluate in its stead (a
// selected attribute, the binding a name refers to); the value is cached here
// either way, so a chain is only ever walked once.
//
// The field order is chosen to keep the struct at 64 bytes, since evaluating
// anything substantial allocates millions of these.
type Expression struct {
	Value  NixValue
	Native *nativeCall
	Scope  *Scope
	Node   *p.Node

	// blame says what a backtrace should call this frame. What it needs
	// besides the kind — the name of an attribute, the function a call
	// belongs to — is a property of the node, and is kept against the node
	// rather than in every expression that mentions it.
	blame   blameKind
	forcing bool
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
func (x *Expression) blaming(kind blameKind) *Expression {
	x.blame = kind
	return x
}

// blamingAttr labels x as the value of an attribute. The name is recorded
// against whatever produces the value, so that the expression itself does not
// have to carry a field for it.
func (x *Expression) blamingAttr(sym Sym) *Expression {
	x.blame = blameAttr
	switch {
	case x.Native != nil:
		x.Native.sym = sym
	case x.Node != nil:
		x.Scope.file.static.get(x.Node.ID).attrSym = sym
	}
	return x
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

// traceFrame renders the frame for a backtrace. What the frame is called, and
// where it points, are read from what the node is known to be: a call points
// at the function rather than at its body, and the value of an attribute is
// named after it.
func (f evalFrame) traceFrame() Frame {
	pr := f.parser()
	pos := f.pos()
	var sym Sym
	if f.native != nil {
		sym = f.native.sym
		if f.native.op != nil {
			sym = f.native.op.Sym
		}
	}
	if f.node != nil && f.scope != nil {
		e := f.scope.file.static.get(f.node.ID)
		if f.blame == blameAttr {
			sym = e.attrSym
		}
		if f.blame == blameCall && e.owner != nil && pr != nil {
			pos = pr.NodePos(e.owner)
		}
	}
	var line string
	if pos != nil {
		line = pr.SourceLine(pos)
	}
	return Frame{Pos: pos, Line: line, Desc: f.blame.describe(sym)}
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
	scope  *Scope
	node   *p.Node
	native *nativeCall
	blame  blameKind
}

// evalStack holds the expressions currently being forced, innermost last. It
// is what an error is annotated from: throwf reads the position and the
// backtrace off it at the point of failure, so that unwinding stays a single
// panic rather than one re-panic per frame.
//
// It is a fixed array indexed by evalDepth rather than a slice appended to.
// Every force pushes a frame, the depth is bounded by maxCallDepth anyway, and
// an indexed store leaves the growth check and the slice header out of the
// hottest function in the evaluator. Evaluation is single-goroutine, so one
// array serves it.
var (
	evalStack [maxCallDepth]evalFrame
	evalDepth int
)

func (x *Expression) force() NixValue {
	if x.forcing {
		// The thunk is already being evaluated further up the stack, so its
		// value is defined in terms of itself.
		throwf(ErrInfiniteRecursion, "infinite recursion encountered")
	}
	x.forcing = true
	depth := evalDepth
	defer x.finish(depth)

	// An expression that only stands in front of another one — a parenthesis,
	// the branch an `if` took, the body of a `let` or of a call — is continued
	// in place rather than delegated to a thunk of its own: resolve points
	// this expression at the next node and the loop goes round again. That is
	// one allocation and one nested force fewer per level, and the backtrace
	// is unchanged because each turn still records a frame.
	for {
		if evalDepth == maxCallDepth {
			throwf(ErrEval, "stack overflow; possible infinite recursion")
		}
		evalStack[evalDepth] = evalFrame{
			scope: x.Scope, node: x.Node, native: x.Native, blame: x.blame,
		}
		evalDepth++

		var lower *Expression
		switch {
		case x.Native != nil:
			x.Value = x.Native.run()
		case x.Node != nil:
			lower = x.resolve()
		default:
			throwf(ErrEval, "expression has nothing to evaluate")
		}
		if x.Value != nil {
			break
		}
		if lower != nil {
			// resolve delegated to an expression that already exists, and
			// which something else may hold. Evaluating it here (rather than
			// continuing in place) keeps every thunk on the stack, so that a
			// cycle through several bindings is detected and each contributes
			// a trace frame.
			x.Value = lower.Eval()
			break
		}
	}
	// The thunk is now nothing but its value, so drop what only producing it
	// needed. This releases the scope chain and the expressions underneath,
	// which otherwise stay reachable for as long as the value does. The nodes
	// are left alone: they belong to the parse tree, which outlives every
	// value anyway, so clearing them would free nothing. Failure leaves
	// everything in place: the expression is still unevaluated, and the error
	// being raised is describing it from the evaluation stack.
	x.Native, x.Scope = nil, nil
	return x.Value
}

// continueAt points the expression at the node to evaluate in its place, in
// the scope that node belongs in. force picks it up from there.
func (x *Expression) continueAt(n *p.Node, scope *Scope) {
	x.Node, x.Scope = n, scope
}

// continueIn is continueAt for a node the backtrace should describe, such as
// the body of a call.
func (x *Expression) continueIn(n *p.Node, scope *Scope, kind blameKind) {
	x.continueAt(n, scope)
	x.blame = kind
}

// finish leaves the expression, whether it produced a value or is being
// unwound past by a failure, dropping every frame it pushed.
func (x *Expression) finish(depth int) {
	x.forcing = false
	evalDepth = depth
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
	x.Native = &nativeCall{fn: f}
	return x
}

// nativeCall is a value produced in Go rather than worked out from syntax: a
// builtin applied to all of its arguments, or a computation the evaluator sets
// up itself. Holding the arguments here rather than in a closure is what keeps
// a builtin call to two objects.
type nativeCall struct {
	op   *NixPrimop
	args [maxPrimopArgs]*Expression
	fn   func() NixValue
	// sym is what a backtrace calls this: the builtin's name, or the
	// attribute the computation stands for.
	sym Sym
}

func (c *nativeCall) run() NixValue {
	if c.op != nil {
		return c.op.Func(c.args[:c.op.ArgNum]...)
	}
	return c.fn()
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
func (x *Expression) evalNodeAs(n *p.Node, kind blameKind) NixValue {
	y := Expression{Scope: x.Scope, Node: n, blame: kind}
	return y.Eval()
}

// thunkFor is the expression to pass where something else will hold on to the
// node rather than evaluate it at once — the argument of a call.
//
// An identifier that the scope chain already holds needs no thunk of its own:
// the binding it names is one, and sharing it is also what makes the argument
// evaluated at most once however many times the callee uses it. A literal is
// the same value every time, so one expression per node serves every call.
// Nix does the same, in Expr::maybeThunk.
//
// What comes back may be shared, so nothing may relabel it for a backtrace:
// whoever holds it already describes it.
func (x *Expression) thunkFor(n *p.Node) *Expression {
	switch n.Type {
	case p.IDNode:
		// A name the chain does not hold may still come from a `with`, whose
		// set has not been evaluated yet, so that one stays a thunk.
		if _, y, ok := x.Scope.lookupNode(n); ok {
			return y
		}
	case p.IntNode, p.FloatNode, p.PathNode, p.URINode:
		return x.Scope.literalExpr(n)
	}
	return x.WithNode(n)
}
