package eval

import (
	"fmt"
	"unsafe"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// NixLambda is anything that can be applied to an argument. Apply is lazy: it
// returns the unevaluated expression standing for the call, so that an
// application only costs what its result is actually used for.
type NixLambda interface {
	Apply(arg *Expression) *Expression
}

// lambdaInfo is everything about a function that its syntax decides. It does
// not depend on the scope the function was created in, so it is worked out
// once per node and shared by every closure made from that node.
type lambdaInfo struct {
	Arg    Sym // `arg: body` or `{ ... }@arg: body`
	HasArg bool

	Formal      map[Sym]*p.Node // formal name to its default, or nil
	FormalOrder []Sym           // formals in source order, for stable errors
	HasFormal   bool
	HasEllipsis bool

	Body *p.Node
	// Node is the function expression itself, which backtraces point at.
	Node *p.Node

	// next is the function this one's body is, for a curried definition like
	// `a: b: …`. Walking that chain is how a call binds several arguments at
	// once, so it is linked up on first use rather than looked up per call.
	next *lambdaInfo
}

// NixExprLambda is a closure: a function written in Nix, together with the
// scope it was defined in. Everything else about it is decided by the syntax
// and shared through lambdaInfo, so making one costs two words.
type NixExprLambda struct {
	*lambdaInfo
	// Scope is the scope the function was defined in. It is held directly
	// rather than through the defining Expression, which drops it once it has
	// been forced.
	Scope *Scope
}

func (f *NixExprLambda) Apply(arg *Expression) *Expression {
	var scope *Scope
	if f.HasFormal {
		binds := NewSet(1 + len(f.FormalOrder))
		scope = f.Scope.Subscope(binds, false)
		if f.HasArg {
			binds.Bind1(f.Arg, arg)
		}
		f.bindFormals(binds, scope, arg)
		binds.finish()
	} else {
		// `arg: body` binds one name, so it needs no map.
		scope = f.Scope.Subscope1(f.Arg, arg)
	}
	// The frame points at the function, which says more than its body would.
	return newScoped(scope, f.Body).blaming(blameCall)
}

// bindFormals matches the argument against `{ a, b ? d, ... }` and adds the
// resulting bindings. Defaults are evaluated in the function's own scope, so
// one formal may refer to another.
func (f *NixExprLambda) bindFormals(binds NixSet, scope *Scope, arg *Expression) {
	val := arg.Eval()
	if val.Kind() != KindSet {
		throwf(ErrType, "value is %s while a set was expected, as the function takes formal arguments",
			anTypeName(val))
	}
	args := val.Set()
	for _, sym := range f.FormalOrder {
		switch y, given := args.Get(sym); {
		case given:
			binds.Bind1(sym, y)
		case f.Formal[sym] != nil:
			binds.Bind1(sym, newScoped(scope, f.Formal[sym]).blamingAttr(sym))
		default:
			throwf(ErrEval, "function called without required argument '%s'", sym)
		}
	}
	if f.HasEllipsis || args.Len() <= len(f.Formal) {
		// Without a surplus there is nothing unexpected to report; the loop
		// above already rejected anything missing.
		return
	}
	unexpected := make([]Sym, 0, args.Len()-len(f.Formal))
	for _, a := range args.attrs {
		if _, ok := f.Formal[a.sym]; !ok {
			unexpected = append(unexpected, a.sym)
		}
	}
	if len(unexpected) != 0 {
		SortSym(unexpected) // report the same one on every run
		throwf(ErrEval, "function called with unexpected argument '%s'", unexpected[0])
	}
}

// maxPrimopArgs is the highest arity in the builtin table. Collecting a
// call's arguments in an array of that size, rather than a slice, is what
// keeps a builtin call to one object for the arguments and one for the
// expression.
const maxPrimopArgs = 3

// NixPrimop is a builtin function. Func is called once ArgNum arguments have
// been collected; until then application yields a NixPartialPrimop.
type NixPrimop struct {
	Func   func(...*Expression) NixValue
	Doc    string
	Sym    Sym
	ArgNum int
}

// printOp is how a builtin renders, which is all a value of one can be asked.
func (op *NixPrimop) printOp() string {
	return fmt.Sprintf("«primop %s»", op.Sym)
}

func (op *NixPrimop) Apply(arg *Expression) *Expression {
	var args [maxPrimopArgs]*Expression
	args[0] = arg
	if op.ArgNum == 1 {
		return op.call(args)
	}
	return newPartial(op, args, 1)
}

func (op *NixPrimop) call(args [maxPrimopArgs]*Expression) *Expression {
	x := newExpr()
	x.a = unsafe.Pointer(&nativeCall{op: op, args: args})
	x.blame = blamePrimop
	return x
}

// NixPartialPrimop is a builtin applied to some but not all of its arguments.
// Args holds the N it has been given so far.
type NixPartialPrimop struct {
	Primop *NixPrimop
	Args   [maxPrimopArgs]*Expression
	N      int
}

func (pp *NixPartialPrimop) printOp() string {
	return fmt.Sprintf("«primop %s, %d of %d arguments»", pp.Primop.Sym, pp.N, pp.Primop.ArgNum)
}

func (pp *NixPartialPrimop) Apply(arg *Expression) *Expression {
	// The arguments are copied rather than extended in place: a partially
	// applied builtin is a value, and may be applied to several arguments.
	args := pp.Args
	args[pp.N] = arg
	if pp.N+1 == pp.Primop.ArgNum {
		return pp.Primop.call(args)
	}
	return newPartial(pp.Primop, args, pp.N+1)
}

// maxSpine is how many arguments of one call are entered together. Beyond
// this the rest are applied one at a time, which is what happened to every
// argument before this existed.
const maxSpine = 8

// applySpine applies a chain of arguments to a function, entering as many
// plain one-name functions as it can in a single step.
//
// `f a b c` is three applications, and taking them one at a time allocates a
// scope for each and a closure for each function in between. Here the scopes
// of one call are allocated together — each already keeps the one before it
// alive, so nothing is retained that was not before — and the closures in
// between are never built at all. It is what Nix's call loop does, and on a
// four-argument recursion it is the difference between five allocations per
// call and two.
//
// It returns the expression standing for the result, or nil when it has
// pointed x at the body to carry on with in place.
func applySpine(x *Expression, fn NixLambda, args []*Expression) *Expression {
	for {
		lam, ok := fn.(*NixExprLambda)
		if !ok || lam.HasFormal {
			// A builtin, or a function matching a formal argument set: those
			// take one argument at a time.
			y := fn.Apply(args[0])
			if len(args) == 1 {
				return y
			}
			fn, args = assertLambda(y.Eval()), args[1:]
			continue
		}

		// How far the run of plain one-name functions goes, and so how many
		// arguments this step binds.
		var infos [maxSpine]*lambdaInfo
		infos[0] = lam.lambdaInfo
		n := 1
		for n < len(args) && infos[n-1].Body.Type == p.FunctionNode {
			prev := infos[n-1]
			if prev.next == nil {
				prev.next = lam.Scope.lambdaInfo(prev.Body)
			}
			if prev.next.HasFormal {
				break
			}
			infos[n], n = prev.next, n+1
		}

		parent := bindArgs(lam.Scope, infos[:n], args[:n])
		body := infos[n-1].Body
		if args = args[n:]; len(args) == 0 {
			// The frame points at the function, which says more than its body
			// would; continuing in place saves the thunk for the body too.
			x.continueIn(body, parent, blameCall)
			return nil
		}
		// More arguments than this run of functions takes, so the body has to
		// be evaluated to find the next one.
		var y Expression
		y.setThunk(parent, body)
		y.blame = blameCall
		fn = assertLambda(y.Eval())
	}
}

// apply2 applies f to two arguments, for the builtins that take a callback.
func apply2(f NixLambda, a, b *Expression) *Expression {
	var x Expression
	args := [2]*Expression{a, b}
	if y := applySpine(&x, f, args[:]); y != nil {
		return y
	}
	// applySpine pointed the scratch expression at the body; give it one of
	// its own, since the caller keeps it.
	y := newExpr()
	*y = x
	return y
}

// partialValue is a partial application together with the expression that
// holds it, which is the only thing ever done with one.
type partialValue struct {
	partial NixPartialPrimop
	expr    Expression
}

// newPartial is a builtin that has been given some of its arguments.
func newPartial(op *NixPrimop, args [maxPrimopArgs]*Expression, n int) *Expression {
	pv := &partialValue{partial: NixPartialPrimop{Primop: op, Args: args, N: n}}
	pv.expr.setValue(NixValue{kind: KindPartial, ptr: unsafe.Pointer(&pv.partial)})
	return &pv.expr
}

// applyToValue applies a function to a value the evaluator has in hand.
func applyToValue(f NixLambda, v NixValue) *Expression {
	return f.Apply(value(v))
}

// bindArgs binds a call's arguments, one scope per argument, and returns the
// innermost. The scopes of one call are allocated together: each keeps the one
// before it alive anyway, so a block retains nothing extra, and a call of
// several arguments costs one allocation rather than one per argument.
func bindArgs(outer *Scope, infos []*lambdaInfo, args []*Expression) *Scope {
	if len(infos) == 1 {
		// One argument is the common case by far, and an object of its own
		// size class is cheaper to get than a slice.
		return outer.Subscope1(infos[0].Arg, args[0])
	}
	scopes := make([]Scope, len(infos))
	parent := outer
	for i := range scopes {
		scopes[i] = Scope{
			sym: infos[i].Arg, bound: unsafe.Pointer(args[i]),
			Parent: parent, file: outer.file,
		}
		parent = &scopes[i]
	}
	return parent
}
