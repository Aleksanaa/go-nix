package eval

import (
	"sync"
	"sync/atomic"
)

// A thunk claimed by one worker and wanted by another is waited for, and
// waiting is where a fault that one worker catches outright can turn into a
// hang instead.
//
// With one worker, a thunk defined in terms of itself is caught the moment the
// claim comes back with our own id. Split across two, neither claim is ours:
// each holds what the other is waiting for, and both wait forever. It is the
// same fault — a value defined in terms of itself — and it has to be the same
// error, so the waiting is watched for a cycle in the graph of who waits for
// what whom.
//
// Nothing here runs unless a claim has already failed, so an evaluation that
// never contends never pays for it.

// waitState is which worker's claim this one is blocked behind, and a counter
// that changes whenever that starts or stops. The counter is what makes a walk
// of the graph trustworthy: a chain of links read one at a time may never have
// existed all at once, so every link is read again afterwards, and only a
// chain whose counters all held still is believed.
//
// It records the worker rather than the thunk deliberately. A thunk pointer
// stored anywhere the heap can reach defeats escape analysis for every
// expression that could reach here — which is all of them, including the ones
// evalNode keeps on the Go stack so that arithmetic allocates nothing per
// operand. Measured: it put allocation back at 17% of a call-heavy run.
type waitState struct {
	holder atomic.Uint32 // whose claim, or zero when not waiting
	gen    atomic.Uint64
}

func (ws *waitState) begin(holder uint32) {
	ws.gen.Add(1)
	ws.holder.Store(holder)
	ws.gen.Add(1)
}

func (ws *waitState) end() {
	ws.gen.Add(1)
	ws.holder.Store(0)
	ws.gen.Add(1)
}

// read takes a consistent pair of whose claim the worker waits behind and
// when, the way a seqlock is read: an odd counter, or one that moved, means
// the pair was taken mid-change and is worth nothing.
func (ws *waitState) read() (uint32, uint64, bool) {
	before := ws.gen.Load()
	if before%2 != 0 {
		return 0, 0, false
	}
	holder := ws.holder.Load()
	if ws.gen.Load() != before {
		return 0, 0, false
	}
	return holder, before, true
}

// workerRegistry maps the id a thunk's claim records back to the worker that
// holds it. Ids start at one, so the slice is indexed by id-1; it only ever
// grows, and is published as a whole, so a reader holding the previous slice
// still finds every worker that existed when it took it.
var workerRegistry struct {
	mu  sync.Mutex
	all atomic.Pointer[[]*worker]
}

// registerWorker gives a worker its id and the slot to match. The id is the
// slot, so the two cannot drift apart: handing out ids from a counter and
// appending separately lets two workers register out of order, and then a
// claim names the wrong worker — which reads as a cycle that is not there.
func registerWorker(w *worker) uint32 {
	workerRegistry.mu.Lock()
	defer workerRegistry.mu.Unlock()
	var all []*worker
	if p := workerRegistry.all.Load(); p != nil {
		all = *p
	}
	all = append(all, w)
	workerRegistry.all.Store(&all)
	return uint32(len(all)) // ids start at one; zero means unclaimed
}

// workerByID is the worker whose claim on a thunk reads as id, or nil when the
// id names no worker — which is what a free or a forced thunk reads as.
func workerByID(id uint32) *worker {
	if id == 0 || id == stateForced {
		return nil
	}
	p := workerRegistry.all.Load()
	if p == nil || int(id) > len(*p) {
		return nil
	}
	return (*p)[id-1]
}

// maxWaitChain bounds how far the graph is followed before giving up. A cycle
// among workers cannot be longer than the number of them, and giving up only
// means waiting a little longer and looking again.
const maxWaitChain = 256

// waitingWouldCycle reports whether this worker, already recorded as waiting
// behind holder's claim, is at the end of a chain of workers each waiting for
// the next one — which is a value defined in terms of itself, spread out.
func (w *worker) waitingWouldCycle(holder uint32) bool {
	type link struct {
		w   *worker
		gen uint64
	}
	var chain [maxWaitChain]link
	n := 0
	for id := holder; n < maxWaitChain; n++ {
		if id == w.id {
			break // the chain came back to us
		}
		h := workerByID(id)
		if h == nil {
			return false // the claim was given up: no cycle, and nothing to wait for
		}
		next, gen, ok := h.wait.read()
		if !ok || next == 0 {
			return false // it is working, not waiting
		}
		chain[n] = link{h, gen}
		id = next
	}
	if n == maxWaitChain {
		return false
	}
	// The chain was read one link at a time and may never have held all at
	// once. It did if nobody in it started or stopped waiting meanwhile.
	for _, l := range chain[:n] {
		if _, gen, ok := l.w.wait.read(); !ok || gen != l.gen {
			return false
		}
	}
	return true
}
