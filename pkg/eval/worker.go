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

	// exprs and scopes are the blocks currently being handed out. A block is
	// retained as long as its liveliest member, which is the trade the slab
	// sizes describe.
	exprs  []Expression
	scopes []Scope

	// memo is where each identifier's binding was found last time, by node
	// id, for the file the worker is currently in. It cannot live against the
	// syntax the way the rest of what is known about a node does: the slot it
	// names is a position in a set built at run time, so it is a fact about
	// this evaluation rather than about the source, and so belongs to whoever
	// is doing it.
	//
	// One file is in hand at a time — the chain of a single evaluation stays
	// in the file it started in — so the current one is cached and the rest
	// are reached through the map.
	memoFile *file
	memo     []lookupMemo
	memos    map[*file][]lookupMemo

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

// lookupMemo is where a name was found last time: how many scopes up, and
// which slot of that scope. Both are one-based, so that zero means nothing is
// remembered; hops of -1 means the file binds the name nowhere, which the
// syntax settles once and for all.
type lookupMemo struct{ hops, slot int32 }

// remember is the worker's memo for a node, growing into the file it is in.
func (w *worker) remember(f *file, id uint32) *lookupMemo {
	if w.memoFile != f {
		w.enterFile(f)
	}
	return &w.memo[id]
}

// enterFile makes f the file the worker's memo is about.
func (w *worker) enterFile(f *file) {
	m, ok := w.memos[f]
	if !ok {
		m = make([]lookupMemo, len(f.static.entries))
		if w.memos == nil {
			w.memos = make(map[*file][]lookupMemo, 1)
		}
		w.memos[f] = m
	}
	w.memoFile, w.memo = f, m
}
