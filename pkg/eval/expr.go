package eval

import (
	"fmt"
	"unsafe"

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
// closes over, holding its value once forced.
//
// The two words a and b are read according to kind, which is what keeps a
// thunk down to 32 bytes — the single most numerous object an evaluation
// allocates, so its size decides how much of a run is spent in the garbage
// collector:
//
//	kind == KindNone, b != nil   a is the scope and b the node to evaluate
//	kind == KindNone, b == nil   a is a builtin call to run
//	kind != KindNone             the value, whose pointer is a and whose
//	                             word — an integer, a list's length — is num
//
// Resolving a node may point the expression at another one to carry on with in
// place, or hand back an expression to evaluate in its stead (a selected
// attribute, the binding a name refers to); the value is kept here either way,
// so a chain is only ever walked once.
type Expression struct {
	a, b unsafe.Pointer
	num  int64
	kind Kind

	// blame says what a backtrace should call this frame. What it needs
	// besides the kind — the name of an attribute, the function a call
	// belongs to — is a property of the node, and is kept against the node
	// rather than in every expression that mentions it.
	blame   blameKind
	forcing bool
}

// Val is the value this expression has been forced to, or no value at all.
func (x *Expression) Val() NixValue {
	return NixValue{ptr: x.a, num: x.num, kind: x.kind}
}

// setValue records the value the expression has been forced to, which also
// releases the scope and the node that produced it: they are no longer part of
// what this expression is.
func (x *Expression) setValue(v NixValue) {
	x.a, x.b, x.num, x.kind = v.ptr, nil, v.num, v.kind
}

// scope is the scope an unforced expression evaluates in.
func (x *Expression) scope() *Scope {
	if x.kind != KindNone || x.b == nil {
		return nil
	}
	return (*Scope)(x.a)
}

// node is the syntax an unforced expression evaluates, or nil for a builtin
// call and for one that has been forced already.
func (x *Expression) node() *p.Node {
	if x.kind != KindNone {
		return nil
	}
	return (*p.Node)(x.b)
}

// native is the builtin call an unforced expression runs, if it is one.
func (x *Expression) native() *nativeCall {
	if x.kind != KindNone || x.b != nil {
		return nil
	}
	return (*nativeCall)(x.a)
}

// setThunk points the expression at a node to evaluate in a scope.
func (x *Expression) setThunk(scope *Scope, n *p.Node) {
	x.a, x.b = unsafe.Pointer(scope), unsafe.Pointer(n)
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
const exprSlabSize = 256

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
	return x.scope().parser()
}

// WithNode derives an unevaluated expression for a sibling node, in the same
// scope.
func (x *Expression) WithNode(n *p.Node) *Expression {
	y := newExpr()
	y.setThunk(x.scope(), n)
	return y
}

// WithScoped derives an unevaluated expression for a node in a new scope.
func (x *Expression) WithScoped(n *p.Node, scope *Scope) *Expression {
	return newScoped(scope, n)
}

// newScoped is an unevaluated expression for a node in a scope.
func newScoped(scope *Scope, n *p.Node) *Expression {
	y := newExpr()
	y.setThunk(scope, n)
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
	if c := x.native(); c != nil {
		c.sym = sym
		return x
	}
	if n := x.node(); n != nil {
		// The same node is bound to the same name on every evaluation, so this
		// is a write to a page that is already right almost every time.
		if e := x.scope().file.static.get(n.ID); e.attrSym != sym {
			e.attrSym = sym
		}
	}
	return x
}

// Pos reports where this expression starts, or nil when it has no syntax (a
// builtin thunk, or a value constructed by Go).
func (x *Expression) Pos() *p.LexPosition {
	pr := x.parser()
	n := x.node()
	if pr == nil || n == nil {
		return nil
	}
	return pr.NodePos(n)
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
	if x.kind != KindNone {
		return x.Val()
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
		// The frame is built inline rather than through the scope, node and
		// native accessors: this is the hottest loop in the evaluator, and
		// here what an unforced expression holds is already known — a and b
		// are set, kind is KindNone, and b distinguishes a node from a
		// builtin call.
		frame := evalFrame{blame: x.blame}
		var lower *Expression
		if x.b != nil {
			frame.scope, frame.node = (*Scope)(x.a), (*p.Node)(x.b)
			evalStack[evalDepth] = frame
			evalDepth++
			lower = x.resolve()
		} else if x.a != nil {
			call := (*nativeCall)(x.a)
			frame.native = call
			evalStack[evalDepth] = frame
			evalDepth++
			x.setValue(call.run())
		} else {
			evalStack[evalDepth] = frame
			evalDepth++
			throwf(ErrEval, "expression has nothing to evaluate")
		}
		if x.kind != KindNone {
			break
		}
		if lower != nil {
			// resolve delegated to an expression that already exists, and
			// which something else may hold. Evaluating it here (rather than
			// continuing in place) keeps every thunk on the stack, so that a
			// cycle through several bindings is detected and each contributes
			// a trace frame.
			x.setValue(lower.Eval())
			break
		}
	}
	// Recording the value is also what releases the scope chain and the
	// expressions underneath it, which would otherwise stay reachable for as
	// long as the value does: the thunk is now nothing but what it evaluated
	// to. Failure leaves everything in place — the expression is still
	// unevaluated, and the error being raised describes it from the evaluation
	// stack.
	return x.Val()
}

// continueAt points the expression at the node to evaluate in its place, in
// the scope that node belongs in. force picks it up from there.
func (x *Expression) continueAt(n *p.Node, scope *Scope) {
	x.setThunk(scope, n)
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
	x.setValue(v)
	return x
}

// thunk wraps a Go computation as an unevaluated expression.
func thunk(f func() NixValue) *Expression {
	x := newExpr()
	x.a = unsafe.Pointer(&nativeCall{fn: f})
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
func (x *Expression) evalNode(n *p.Node) NixValue { return x.scope().evalNode(n) }

// evalNodeAs is evalNode for an operand that a backtrace should name.
func (x *Expression) evalNodeAs(n *p.Node, kind blameKind) NixValue {
	var y Expression
	y.setThunk(x.scope(), n)
	y.blame = kind
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
		if _, y, ok := x.scope().lookupNode(n); ok {
			return y
		}
	case p.IntNode, p.FloatNode, p.PathNode, p.URINode:
		return x.scope().literalExpr(n)
	}
	return x.WithNode(n)
}
