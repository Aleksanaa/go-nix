package eval

import (
	"fmt"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// NixLambda is a value that can be applied to an argument. Apply is lazy: it
// returns the unevaluated expression standing for the call, so that an
// application only costs what its result is actually used for.
type NixLambda interface {
	NixValue
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

func (f *NixExprLambda) Print(recurse int) string { return "«lambda»" }

// Compare is always false: Nix cannot compare functions.
func (f *NixExprLambda) Compare(val NixValue) bool { return false }

func (f *NixExprLambda) Apply(arg *Expression) *Expression {
	var scope *Scope
	if f.HasFormal {
		binds := make(NixSet, 1+len(f.FormalOrder))
		scope = f.Scope.Subscope(binds, false)
		if f.HasArg {
			binds[f.Arg] = arg
		}
		f.bindFormals(binds, scope, arg)
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
	args, ok := arg.Eval().(NixSet)
	if !ok {
		throwf(ErrType, "value is %s while a set was expected, as the function takes formal arguments",
			anTypeName(arg.Value))
	}
	for _, sym := range f.FormalOrder {
		switch y, given := args[sym]; {
		case given:
			binds[sym] = y
		case f.Formal[sym] != nil:
			binds[sym] = newScoped(scope, f.Formal[sym]).blamingAttr(sym)
		default:
			throwf(ErrEval, "function called without required argument '%s'", sym)
		}
	}
	if f.HasEllipsis || len(args) <= len(f.Formal) {
		// Without a surplus there is nothing unexpected to report; the loop
		// above already rejected anything missing.
		return
	}
	unexpected := make([]Sym, 0, len(args)-len(f.Formal))
	for sym := range args {
		if _, ok := f.Formal[sym]; !ok {
			unexpected = append(unexpected, sym)
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

func (op *NixPrimop) Print(recurse int) string {
	return fmt.Sprintf("«primop %s»", op.Sym)
}

func (op *NixPrimop) Compare(val NixValue) bool { return false }

func (op *NixPrimop) Apply(arg *Expression) *Expression {
	var args [maxPrimopArgs]*Expression
	args[0] = arg
	if op.ArgNum == 1 {
		return op.call(args)
	}
	return value(&NixPartialPrimop{Primop: op, Args: args, N: 1})
}

func (op *NixPrimop) call(args [maxPrimopArgs]*Expression) *Expression {
	x := newExpr()
	x.Native = &nativeCall{op: op, args: args}
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

func (pp *NixPartialPrimop) Print(recurse int) string {
	return fmt.Sprintf("«primop %s, %d of %d arguments»", pp.Primop.Sym, pp.N, pp.Primop.ArgNum)
}

func (pp *NixPartialPrimop) Compare(val NixValue) bool { return false }

func (pp *NixPartialPrimop) Apply(arg *Expression) *Expression {
	// The arguments are copied rather than extended in place: a partially
	// applied builtin is a value, and may be applied to several arguments.
	args := pp.Args
	args[pp.N] = arg
	if pp.N+1 == pp.Primop.ArgNum {
		return pp.Primop.call(args)
	}
	return value(&NixPartialPrimop{Primop: pp.Primop, Args: args, N: pp.N + 1})
}

// apply2 applies f to two arguments at once.
//
// A function written as `a: b: body` — the shape a curried call reaches, and
// the one every builtin that takes a callback expects — is then entered once:
// the closure that would stand for the function in between the two arguments
// is never built, and neither is the expression that would force it.
// Anything else falls back to applying one argument after the other.
func apply2(f NixLambda, a, b *Expression) *Expression {
	if lam, ok := f.(*NixExprLambda); ok && !lam.HasFormal && lam.Body.Type == p.FunctionNode {
		if inner := lam.Scope.lambdaInfo(lam.Body); !inner.HasFormal {
			scope := lam.Scope.Subscope1(lam.Arg, a).Subscope1(inner.Arg, b)
			return newScoped(scope, inner.Body).blaming(blameCall)
		}
	}
	return assertLambda(f.Apply(a).Eval()).Apply(b)
}

// applyIn enters a function in the calling expression itself: binding the
// argument makes a scope, and the body is evaluated in place of the call. It
// reports whether it could — a builtin, or a function taking a formal argument
// set, still needs an expression of its own.
func applyIn(x *Expression, f NixLambda, arg *Expression) bool {
	lam, ok := f.(*NixExprLambda)
	if !ok || lam.HasFormal {
		return false
	}
	// The frame points at the function, which says more than its body would.
	x.continueIn(lam.Body, lam.Scope.Subscope1(lam.Arg, arg), blameCall)
	return true
}

// applyIn2 is applyIn for `f a b`, where the function is written `a: b: body`.
// Both arguments are bound before the body is entered, so the closure that
// would stand for the function in between them is never built.
func applyIn2(x *Expression, f NixLambda, a, b *Expression) bool {
	lam, ok := f.(*NixExprLambda)
	if !ok || lam.HasFormal || lam.Body.Type != p.FunctionNode {
		return false
	}
	inner := lam.Scope.lambdaInfo(lam.Body)
	if inner.HasFormal {
		return false
	}
	scope := lam.Scope.Subscope1(lam.Arg, a).Subscope1(inner.Arg, b)
	x.continueIn(inner.Body, scope, blameCall)
	return true
}
