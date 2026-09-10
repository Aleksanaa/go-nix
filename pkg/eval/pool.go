package eval

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
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

// worthForking reports whether n items are worth handing out, and turns the
// parallel mode on if they are.
//
// It asks about the caller's budget as well as the length, so that a worker
// which was itself forked does not fork again: see forceAll for what happens
// when it can. A caller that has to build the expressions before there is
// anything to hand out asks this first, rather than building them and finding
// out afterwards.
func worthForking(w *worker, n int) bool {
	return n >= forkMinItems && w.budget >= 2 && goParallel()
}

// Waiting for a worker we forked, and why it is never done while holding a
// claim.
//
// A worker blocked behind a claim is recorded in the wait graph, and a cycle
// there is caught and reported as the recursion it is. A worker blocked on a
// channel is recorded nowhere: it is behind no claim, so nothing links it to
// the worker it waits for. Wait for a fork while still holding claims and the
// two meet — the forked worker blocks behind a claim of ours, we block on its
// channel, and the cycle has an edge the graph cannot see. Both fork points
// hung on that before they were written this way round.
//
// So the value a fork produces is never taken from the channel. It is taken by
// forcing the thunk, which is the one wait that is watched: if the forked
// worker still holds the claim, we block behind it in the graph like anyone
// else, and a deadlock against us is a cycle like any other.
//
// What is left to wait for afterwards is only the forked worker's way out,
// which needs no claim of ours and so cannot block on one. That wait is safe,
// and it is what keeps the pool from growing without bound.
//
// It is skipped where a failure is unwinding past it, which is why these waits
// are called rather than deferred. The claims this worker holds are released as
// it unwinds, so a forked worker blocked behind one is freed by the very
// unwinding that would otherwise be waiting for it, and finishes on its own
// with whatever it was forcing memoized.

// liveForks counts the workers that have been forked and have not finished.
//
// An evaluation never reads it: a fork it walked away from finishes by itself,
// and what that fork forces is memoized rather than lost. It is here for the
// tests, which do need to know. The knobs above are written where the process
// starts and read on the evaluation path, so a test that puts one back has to
// wait for the workers an earlier failure abandoned — otherwise it writes one
// while an abandoned worker is still reading it, which is a race, and the race
// detector duly finds it.
var liveForks atomic.Int64

// forkBegin records a forked worker and returns the call that records its end.
// Both are here rather than at the fork points so that neither can be written
// without the other, and it is called before the goroutine starts rather than
// inside it, so that a count taken in between is not short.
func forkBegin() func() {
	liveForks.Add(1)
	return func() { liveForks.Add(-1) }
}

// forceAll starts forcing every expression in xs across the pool, for a caller
// that is about to force them all itself, and hands back the wait for the pool
// to be done with them.
//
// With one worker it does nothing at all: the caller's own loop is the
// forcing, in the order the language says. With several it is a head start,
// and the caller's loop then finds the values already there.
//
// It spends the caller's budget to do it, as the operand fork does, and a
// worker that was itself forked has none. Without that a chunk could fork
// chunks of its own: a set built out of sets would hand out parWorkers workers
// per level, and — worse — a failure could go round in circles. A cycle
// through a forked list throws and releases the claim, an abandoned chunk
// picks the same thunk up, forks a fresh set of chunks, and fails the same way
// again, for as long as there is memory to make workers in. That ran the
// machine out of it.
func forceAll(w *worker, xs []*Expression) func() {
	if !worthForking(w, len(xs)) {
		return func() {}
	}
	return forkRange(len(xs), func(w *worker, lo, hi int) {
		for _, x := range xs[lo:hi] {
			x.Eval(w)
		}
	})
}

// forkRange runs body over [0,n) split into contiguous chunks, one per worker,
// and hands back the wait for all of them. A chunk that fails is abandoned:
// whatever it forced stays forced, and the caller meets the failure in its own
// order.
func forkRange(n int, body func(w *worker, lo, hi int)) func() {
	p := min(parWorkers, n)
	chunk := (n + p - 1) / p
	var wg sync.WaitGroup
	for lo := 0; lo < n; lo += chunk {
		hi := min(lo+chunk, n)
		wg.Add(1)
		end := forkBegin()
		go func() {
			defer wg.Done()
			defer end()
			w := takeWorker()
			defer dropWorker(w)
			// A failure here is not this pass's to report.
			catching(func() struct{} { body(w, lo, hi); return struct{}{} })
		}()
	}
	return wg.Wait
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
// that force whole lists. It is separate from GON_PAR, and still off, though
// no longer because it is dangerous:
//
//	                default  GON_PAR_OPS=1   nix
//	hanoi-calls          247             50   190
//	lookup               118            137    86
//	lazy                 122            137    82
//	hanoi                116            135    78
//	attrs                 75             79    60
//
// The win is a recursion that splits into two large independent halves, which
// is what a fork is for. The loss used to be a fold whose operand happens to
// be a call — `acc + builtins.getAttr name set` — where the work handed over
// was over before the goroutine that took it had started, fifty thousand
// times; that cost `attrs` 87ms → 204ms until the pass began marking calls
// that reach the function they are written inside, and only those are forked.
//
// What is left is smaller and of a different kind. `hanoi` forks by the mark
// and gains nothing: its halves are lists, and the `++` that joins them copies
// both, so the concatenation sits on the critical path however cheap the
// halves become. `lookup` and `lazy` barely fork at all and lose anyway,
// because one fork anywhere turns goParallel on for the rest of the run and
// the atomic claim is then paid everywhere.
//
// So the question is still the one marking could not answer: not which
// applications are worth forking, but how much work one turns out to be. Only
// evaluating it once says, and the worker's per-node memo from D1 is where a
// count of that would go.
var forkOps = os.Getenv("GON_PAR_OPS") == "1"
