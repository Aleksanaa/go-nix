package eval

import (
	"sync"
	"testing"
	"time"
)

// TestCrossWorkerCycleIsInfiniteRecursion pins the case that a claim on a
// thunk introduces and one worker never could: two workers each holding what
// the other is waiting for.
//
// With one worker a thunk defined in terms of itself is caught the moment the
// claim comes back with our own id. Split across two, neither claim is ours,
// so both wait — and what was an error becomes a hang. A cycle in the graph of
// who waits for whom is exactly the same fault, and must be the same error.
//
// The cycle is built by hand rather than out of Nix source, because arranging
// for two workers to claim the two halves of a recursion before either gets
// further is not something the language can be asked for.
func TestCrossWorkerCycleIsInfiniteRecursion(t *testing.T) {
	evaluatingInParallel(t)

	var x, y *Expression
	xHeld, yHeld := make(chan struct{}), make(chan struct{})
	var xOnce, yOnce sync.Once

	// Each thunk announces that its worker has claimed it, waits for the other
	// to do the same, and only then reaches for the other — so both claims are
	// held before either wait begins. Announcing is idempotent because a thunk
	// released by a failure is forced again by whoever was waiting for it.
	x = thunk(mainWorker, func(w *worker) NixValue {
		xOnce.Do(func() { close(xHeld) })
		<-yHeld
		return y.Eval(w)
	})
	y = thunk(mainWorker, func(w *worker) NixValue {
		yOnce.Do(func() { close(yHeld) })
		<-xHeld
		return x.Eval(w)
	})

	errs := make(chan error, 2)
	for _, start := range []*Expression{x, y} {
		go func() {
			w := newWorker()
			_, err := catching(func() NixValue { return start.Eval(w) })
			errs <- err
		}()
	}

	for range 2 {
		select {
		case err := <-errs:
			if err == nil {
				continue // one of them may legitimately get a value
			}
			if e, ok := err.(*EvalError); !ok || e.Kind != ErrInfiniteRecursion {
				t.Errorf("got %v, want an infinite-recursion error", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the two workers deadlocked waiting for each other")
		}
	}
}
