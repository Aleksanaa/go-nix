package eval

import "sync/atomic"

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
	// id names this worker among all that are evaluating at once. It is what
	// a thunk records when it is claimed, so that the worker holding it can be
	// told apart from the ones waiting on it.
	id uint32

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
	// this evaluation rather than about the source. Keeping it per worker is
	// what lets several of them look names up at once.
	//
	// One file is in hand at a time — the chain of a single evaluation stays
	// in the file it started in — so the current one is cached and the rest
	// are reached through the map.
	memoFile *file
	memo     []lookupMemo
	memos    map[*file][]lookupMemo
}

// w is the worker every evaluation runs on. Phase D replaces it with one per
// goroutine, threaded rather than named; until then the indirection is the
// whole of the change, and is measured.
var mainWorker = newWorker()

// workerIDs hands out ids to workers, so that a thunk's claim names its owner
// unambiguously. Zero is reserved for "unclaimed".
var workerIDs atomic.Uint32

// newWorker makes a worker with an id of its own. The id is what a thunk
// records when the worker claims it, so it must never be zero.
func newWorker() *worker {
	id := workerIDs.Add(1)
	if id == 0 || id == stateForced {
		panic("eval: ran out of worker ids")
	}
	return &worker{id: id}
}

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

// parallel says whether more than one worker may be evaluating. It is settled
// before an evaluation starts and does not change while one runs.
//
// It exists because the claim on a thunk is only worth synchronising when
// there is somebody to synchronise with. Publishing a value with an atomic
// store costs about a tenth of a run — on amd64 it is a locked exchange, and
// it happens once per force — and buys nothing at all while one goroutine is
// doing everything. With one worker the claim word is touched plainly; with
// several, every touch is atomic, so all of them agree. Mixing the two is safe
// only because the mode is fixed for the whole evaluation.
var parallel bool
