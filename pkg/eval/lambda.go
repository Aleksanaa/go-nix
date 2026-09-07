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

// NixExprLambda is a closure: a function written in Nix, together with the
// scope it was defined in.
type NixExprLambda struct {
	Arg    Sym // `arg: body` or `{ ... }@arg: body`
	HasArg bool

	Formal      map[Sym]*p.Node // formal name to its default, or nil
	FormalOrder []Sym           // formals in source order, for stable errors
	HasFormal   bool
	HasEllipsis bool

	Body *p.Node
	// Scope is the scope the function was defined in, and Node the function
	// expression itself, which backtraces point at. They are held directly
	// rather than through the defining Expression, which drops them once it
	// has been forced.
	Scope *Scope
	Node  *p.Node
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
	return newScoped(scope, f.Body).blamingAt(blameCall, 0, f.Node)
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
			binds[sym] = newScoped(scope, f.Formal[sym]).blaming(blameAttr, sym)
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
	if op.ArgNum == 1 {
		return op.call([]*Expression{arg})
	}
	return value(&NixPartialPrimop{Primop: op, Args: []*Expression{arg}})
}

func (op *NixPrimop) call(args []*Expression) *Expression {
	return thunk(func() NixValue { return op.Func(args...) }).blaming(blamePrimop, op.Sym)
}

// NixPartialPrimop is a builtin applied to some but not all of its arguments.
type NixPartialPrimop struct {
	Primop *NixPrimop
	Args   []*Expression
}

func (pp *NixPartialPrimop) Print(recurse int) string {
	return fmt.Sprintf("«primop %s, %d of %d arguments»", pp.Primop.Sym, len(pp.Args), pp.Primop.ArgNum)
}

func (pp *NixPartialPrimop) Compare(val NixValue) bool { return false }

func (pp *NixPartialPrimop) Apply(arg *Expression) *Expression {
	// Copy rather than append in place: a partially applied builtin is a value
	// that may be applied to several different arguments.
	args := make([]*Expression, len(pp.Args), len(pp.Args)+1)
	copy(args, pp.Args)
	args = append(args, arg)
	if len(args) == pp.Primop.ArgNum {
		return pp.Primop.call(args)
	}
	return value(&NixPartialPrimop{Primop: pp.Primop, Args: args})
}
