package eval

// A worker is the state an evaluation keeps that belongs to whoever is doing
// the evaluating rather than to the expressions being evaluated: the stack a
// backtrace is read from, and the blocks new expressions and scopes are handed
// out of. Everything else — values, thunks, scopes, the facts cached against
// syntax — belongs to the evaluation itself and is shared.
//
// It is gathered into one place so that there can be more than one of it.
// Evaluation is still single-goroutine and there is still exactly one worker;
// what changes is that no part of the evaluator now names a package variable
// to find this state, which is the first thing standing between here and
// forcing thunks in parallel. See PLAN.md, phase D.
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

	// exprs and scopes are the blocks currently being handed out. A block is
	// retained as long as its liveliest member, which is the trade the slab
	// sizes describe.
	exprs  []Expression
	scopes []Scope
}

// w is the worker every evaluation runs on. Phase D replaces it with one per
// goroutine, threaded rather than named; until then the indirection is the
// whole of the change, and is measured.
var mainWorker = &worker{}

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

// newScope returns a zeroed scope from the block being handed out.
func (w *worker) newScope() *Scope {
	if scopeSlabSize <= 1 {
		return new(Scope)
	}
	if len(w.scopes) == 0 {
		w.scopes = make([]Scope, scopeSlabSize)
	}
	s := &w.scopes[0]
	w.scopes = w.scopes[1:]
	return s
}
