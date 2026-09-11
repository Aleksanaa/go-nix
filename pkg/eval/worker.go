package eval

import "unsafe"

// A worker is the state an evaluation keeps that belongs to whoever is doing
// the evaluating rather than to the expressions being evaluated: the stack a
// backtrace is read from, and the blocks new expressions and scopes are handed
// out of. Everything else — values, thunks, scopes, the facts cached against
// syntax — belongs to the evaluation itself and is shared.
//
// It is gathered into one place rather than kept in package variables, so that
// nothing in the evaluator reaches past the worker it was handed. Evaluation
// runs on one goroutine; this is what an evaluation would be threaded through
// if it ever ran on more than one.
type worker struct {
	// stack holds the expressions currently being forced, innermost last. It
	// is what an error is annotated from: throwf reads the position and the
	// backtrace off it at the point of failure, so that unwinding stays a
	// single panic rather than one re-panic per frame.
	//
	// It is a fixed array indexed by depth rather than a slice appended to.
	// Every force pushes a frame, the depth is bounded by maxCallDepth
	// anyway, and an indexed store leaves the growth check and the slice
	// header out of the hottest function in the evaluator.
	stack [maxCallDepth]evalFrame
	depth int

	// exprs, envs and slots are the blocks currently being handed out. A block
	// is retained as long as its liveliest member, which is the trade the slab
	// sizes describe.
	exprs []Expression
	envs  []Env
	slots []*Expression

	// seen is the sets and lists already printed in the current Print
	// traversal, so that a value that refers back to itself — a derivation's
	// `out`, which is the derivation again — is reported as "«repeated»"
	// rather than recursing without end. It is only kept for a full print,
	// which is the one traversal the depth bound cannot stop; see Print.
	seen map[unsafe.Pointer]bool

	// base is the scope an import evaluates its file in: the default scope,
	// shared by every worker. It lives on the worker rather than being named
	// from a builtin so that the builtins map can be initialised before the
	// default scope is built.
	base *Scope
}

// mainWorker is the worker every evaluation runs on.
var mainWorker = newWorker()

// newWorker makes a worker to evaluate on.
func newWorker() *worker { return &worker{} }

// newExpr returns a zeroed expression from the block being handed out.
func (w *worker) newExpr() *Expression {
	if exprSlabSize <= 1 {
		return new(Expression)
	}
	if len(w.exprs) == 0 {
		w.exprs = make([]Expression, exprSlabSize)
	}
	x := &w.exprs[0]
	w.exprs = w.exprs[1:]
	return x
}

// newEnv returns a zeroed frame from the block being handed out.
func (w *worker) newEnv() *Env {
	if len(w.envs) == 0 {
		w.envs = make([]Env, envSlabSize)
	}
	e := &w.envs[0]
	w.envs = w.envs[1:]
	return e
}

// newSlots returns n empty slots from the block being handed out. A frame's
// slots outlive nothing the frame does not, so they come from a block of their
// own rather than one allocation each.
func (w *worker) newSlots(n int) []*Expression {
	if n > envSlabSize {
		return make([]*Expression, n)
	}
	if len(w.slots) < n {
		w.slots = make([]*Expression, envSlabSize)
	}
	s := w.slots[:n:n]
	w.slots = w.slots[n:]
	return s
}
