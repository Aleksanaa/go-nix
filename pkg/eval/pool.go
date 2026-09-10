package eval

import (
	"os"
	"runtime"
	"strconv"
	"sync"
)

// Forcing thunks on more than one goroutine.
//
// Where to do it is the whole question, and the answer is not "wherever there
// is a list". Evaluation is lazy, so forcing something the evaluation would
// not have forced turns a working program into a failing or a hanging one.
// What is safe is the handful of builtins that force every element of a list
// *by necessity* — listToAttrs has to read a name out of each, filter has to
// ask about each — where the work is going to happen anyway and only its order
// changes.
//
// Order is still observable through errors, and that is handled by not
// propagating them: the parallel pass forces what it can and swallows what
// fails, and the ordinary sequential loop that follows walks the same list and
// meets the same failure in the order it would have, with the backtrace it
// would have had. What the parallel pass leaves behind is the work already
// done, memoized in the thunks.
//
// The one thing that changes is that an element the sequential evaluation
// would never have reached — one after the first failure — is evaluated. In a
// pure language that is unobservable unless it fails to terminate, and Nix has
// no unbounded loop except recursion, which the depth limit ends.

// parWorkers is how many goroutines may force thunks at once. It is not how
// many will: see goParallel.
//
// The default is the machine's, capped, because past a handful the sweep on
// the example workloads stops improving and starts costing — `lists` and
// `hanoi-calls` both bottom out around eight and are worse at sixteen. GON_PAR
// overrides it, and one means the evaluator behaves exactly as it did before
// there were any workers.
var parWorkers = func() int {
	if n, err := strconv.Atoi(os.Getenv("GON_PAR")); err == nil && n >= 1 {
		return n
	}
	return min(runtime.NumCPU(), 8)
}()

// goParallel turns on the mode that makes a thunk's claim atomic, and reports
// whether the evaluation may fork. It is called where a fork is about to
// happen and not before, so an evaluation that never reaches a fork point
// never pays for one: workloads shaped like a fold have no parallel work to
// find, and asking for the atomics up front made them 10% slower for nothing.
//
// Turning it on part-way through is safe for exactly one reason: until the
// first fork there is only one goroutine, and starting one is an ordering
// edge, so every claim written plainly before this point is visible to every
// worker after it. It is one-way — once anybody may be forcing, everybody has
// to agree — and this is the only place that sets it.
func goParallel() bool {
	if parallel {
		return true
	}
	if parWorkers < 2 {
		return false
	}
	parallel = true
	return true
}

// forkMinItems is the shortest list worth handing out. Below it the goroutines
// and the waiting cost more than the forcing does.
const forkMinItems = 64

// forceAll forces every expression in xs across the pool, for a caller that is
// about to force them all itself.
//
// With one worker it does nothing at all: the caller's own loop is the forcing,
// in the order the language says. With several it is a head start, and the
// caller's loop then finds the values already there.
func forceAll(w *worker, xs []*Expression) {
	if len(xs) < forkMinItems || !goParallel() {
		return
	}
	forkRange(len(xs), func(w *worker, lo, hi int) {
		for _, x := range xs[lo:hi] {
			x.Eval(w)
		}
	})
}

// forkRange runs body over [0,n) split into contiguous chunks, one per worker,
// and waits for all of them. A chunk that fails is abandoned: whatever it
// forced stays forced, and the caller meets the failure in its own order.
func forkRange(n int, body func(w *worker, lo, hi int)) {
	p := min(parWorkers, n)
	chunk := (n + p - 1) / p
	var wg sync.WaitGroup
	for lo := 0; lo < n; lo += chunk {
		hi := min(lo+chunk, n)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := takeWorker()
			defer dropWorker(w)
			// A failure here is not this pass's to report.
			catching(func() struct{} { body(w, lo, hi); return struct{}{} })
		}()
	}
	wg.Wait()
}

// Workers are reused rather than made per chunk: one carries a backtrace stack
// of maxCallDepth frames, which is most of a megabyte, and making one per
// chunk would cost more than the forcing saves.
var workerPool = sync.Pool{New: func() any { return newWorker() }}

func takeWorker() *worker { return workerPool.Get().(*worker) }

func dropWorker(w *worker) {
	// The chunk is over, so nothing is being forced and nothing is waited on;
	// what is left is only the blocks it was handing out, which the next user
	// of this worker carries on with.
	w.depth, w.budget = 0, 0
	workerPool.Put(w)
}

// The worker every evaluation starts on may split its work as far as the pool
// allows; the ones it splits onto get half of what is left each.
func init() { mainWorker.budget = parWorkers }

// forkOps says whether operator operands are forked as well as the builtins
// that force whole lists. It is separate from GON_PAR, and off, because
// whether it wins depends entirely on the shape of the code:
//
//	hanoi-calls  274ms → 53ms at eight workers, against nix's 192ms
//	lists         84ms → 53ms                            nix's  62ms
//	attrs         87ms → 204ms
//	fix          126ms → 215ms
//
// The win is a recursion that splits into two large independent halves, which
// is what a fork is for. The loss is a fold whose operand happens to be a call
// — `acc + builtins.getAttr name set` — where the work handed over is over
// before the goroutine that took it has started, fifty thousand times.
//
// Telling those apart is the missing piece, and it is not the syntax: both are
// an application in an operand. It is how much work the call turns out to be,
// which only evaluating it once says. The next step is to have the pass mark a
// call that reaches the function it is written inside — a recursive call, the
// shape that divides and conquers — and fork only those.
var forkOps = os.Getenv("GON_PAR_OPS") == "1"
