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
			assertInfiniteRecursion(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("the two workers deadlocked waiting for each other")
		}
	}
}

// forking turns both fork points on for the duration of a test, and waits for
// the workers they leave behind before putting them back.
//
// The waiting is the part that is easy to miss. These knobs are read on the
// evaluation path and written where the process starts, which is what lets
// them be plain variables, and a test may only write them under the same rule:
// while nothing is evaluating. A failure walks away from the workers it forked
// rather than waiting for them — see the note on waiting for a fork in pool.go
// — so "the test returned" is not "nothing is evaluating", and a restore that
// assumed it was raced against the workers the test had abandoned.
func forking(t *testing.T) {
	t.Helper()
	wasWorkers := parWorkers
	parWorkers = max(parWorkers, 2)
	t.Cleanup(func() {
		waitForForks(t)
		parWorkers = wasWorkers
	})
}

// waitForForks blocks until no forked worker is left running. It sleeps rather
// than spinning: the workers it waits for are still doing the forcing, and
// taking a processor away from them to ask again is how a wait for other
// people's work turns into a livelock.
func waitForForks(t *testing.T) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); liveForks.Load() != 0; {
		if time.Now().After(deadline) {
			t.Errorf("%d forked workers never finished", liveForks.Load())
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// evalWithin evaluates src, failing rather than hanging if it does not finish.
// Every fault here is one whose broken form is a deadlock, so a test for it
// that merely takes a long time would be a test that never reports.
func evalWithin(t *testing.T, src string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := EvalString(src)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatal("evaluation deadlocked")
		return nil
	}
}

func assertInfiniteRecursion(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	if e, ok := err.(*EvalError); !ok || e.Kind != ErrInfiniteRecursion {
		t.Errorf("got %v, want an infinite-recursion error", err)
	}
}

// TestForkCycleIsInfiniteRecursion pins the deadlock a fork point introduces
// when it collects its result from the goroutine rather than from the thunk.
//
// A worker blocked behind a claim is in the wait graph, where a cycle is
// caught. A worker blocked on a channel is in no graph at all. Wait for a fork
// on its channel while still holding claims and the forked worker can block
// behind one of them, which is a cycle with an edge nothing can see: it hangs,
// where one worker would have reported the recursion outright.
//
// Both cases below are that fault, one per fork point, and both hung before
// the waiting was turned around. Neither is contrived: each is a value defined
// in terms of itself, which is the error the sequential evaluation gives.
func TestForkCycleIsInfiniteRecursion(t *testing.T) {
	t.Run("operand", func(t *testing.T) {
		forking(t)
		// `g n` is a recursive call, so it is the forked operand, and the
		// forked worker reaches `a` — the binding this worker is still
		// forcing, and whose claim it therefore blocks behind.
		assertInfiniteRecursion(t, evalWithin(t, `let a = g 1; g = n: a + g n; in a`))
	})

	t.Run("list", func(t *testing.T) {
		forking(t)
		// One element of the list listToAttrs is forcing across the pool needs
		// the set that listToAttrs is itself the value of. The chunk that
		// holds that element blocks behind this worker's claim on `s`.
		assertInfiniteRecursion(t, evalWithin(t, `
			let
			  s = builtins.listToAttrs xs;
			  xs = builtins.genList (i:
			    if i == 50
			    then builtins.head (builtins.attrValues s)
			    else { name = "k${toString i}"; value = i; }) 100;
			in s`))
	})
}

// TestForkedFailureKeepsSourceOrder pins what a fork may not change: which
// error an expression fails with.
//
// A fork evaluates what the sequential evaluation might never have reached, so
// it can meet a failure that is not the one to report. What keeps that from
// being observable is that the parallel pass swallows every failure it meets,
// and the ordinary loop that follows walks the same ground and meets the real
// one in the order the language says — on this worker, with the backtrace it
// would have had.
func TestForkedFailureKeepsSourceOrder(t *testing.T) {
	forking(t)
	for _, test := range []struct{ src, want string }{
		// Both operands fail. The left one is the failure, and the forked
		// right one is swallowed.
		{`let h = n: if n == 0 then throw "left-0"
		             else (throw "left-${toString n}") + h (n - 1); in h 3`, "left-3"},

		// Only the forked operand fails, so its error is the one to report --
		// which means raising it again here rather than passing it back.
		{`let h = n: if n == 0 then throw "right-end" else 0 + h (n - 1); in h 3`, "right-end"},

		// Two elements of a forked list fail, and the chunks meet them in
		// whichever order they run. The earlier is still the failure.
		{`builtins.listToAttrs (builtins.genList (i:
		    if i == 10 then throw "ten"
		    else if i == 20 then throw "twenty"
		    else { name = "k${toString i}"; value = i; }) 100)`, "ten"},

		{`builtins.filter (i:
		    if i == 10 then throw "ten"
		    else if i == 20 then throw "twenty"
		    else true) (builtins.genList (x: x) 100)`, "ten"},
	} {
		err := evalWithin(t, test.src)
		e, ok := err.(*EvalError)
		if !ok {
			t.Errorf("got %v, want an *EvalError", err)
			continue
		}
		if e.Msg != test.want {
			t.Errorf("failed with %q, want %q — a fork reordered the failure", e.Msg, test.want)
		}
		// The error is raised again on this worker, so it carries this
		// worker's stack; the one the fork swallowed carried the fork's.
		if len(e.Trace) == 0 {
			t.Errorf("%q came back without a backtrace", e.Msg)
		}
	}
}

// TestSetForceKeepsSourceOrder pins the same guarantee for the places that
// force every value of a set — deepSeq and toJSON — which is the shape a
// package's dependency closure has. The forked pass swallows, and the ordinary
// loop meets the real failure in order, so the earlier attribute is the one
// reported even when a later one fails too.
func TestSetForceKeepsSourceOrder(t *testing.T) {
	forking(t)
	for _, test := range []struct{ src, want string }{
		{`builtins.deepSeq (builtins.listToAttrs (builtins.genList (i:
		    if i == 10 then { name = "k${toString i}"; value = throw "ten"; }
		    else if i == 20 then { name = "k${toString i}"; value = throw "twenty"; }
		    else { name = "k${toString i}"; value = i; }) 100)) 1`, "ten"},
		{`builtins.stringLength (builtins.toJSON (builtins.listToAttrs (builtins.genList (i:
		    if i == 10 then { name = "k${toString i}"; value = throw "ten"; }
		    else if i == 20 then { name = "k${toString i}"; value = throw "twenty"; }
		    else { name = "k${toString i}"; value = i; }) 100)))`, "ten"},
	} {
		err := evalWithin(t, test.src)
		e, ok := err.(*EvalError)
		if !ok {
			t.Errorf("got %v, want an *EvalError", err)
			continue
		}
		if e.Msg != test.want {
			t.Errorf("failed with %q, want %q — a fork reordered the failure", e.Msg, test.want)
		}
		if len(e.Trace) == 0 {
			t.Errorf("%q came back without a backtrace", e.Msg)
		}
	}
}

// TestFixpointForcesInParallel pins that forking a set's values does not
// disturb a fixpoint. `fix = f: let x = f x; in x` materialises to a set
// before its values are forced, so a value reaching back into the set — here
// every value reads `self.p0.idx` — is a DAG edge, not a cycle. The parallel
// pass must force all 64 values and resolve every cross-reference exactly as
// the sequential walk would: the sum of `idx + buddy` is 0..63 = 2016.
func TestFixpointForcesInParallel(t *testing.T) {
	forking(t)
	got, err := evalPrint(t, `let
	  fix = f: let x = f x; in x;
	  mk = self: builtins.listToAttrs (builtins.genList (i: {
	    name = "p${toString i}";
	    value = { idx = i; buddy = self.p0.idx; };
	  }) 64);
	  set = fix mk;
	in builtins.deepSeq set (builtins.foldl' (a: p: a + p.idx + p.buddy) 0 (builtins.attrValues set))`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2016"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
