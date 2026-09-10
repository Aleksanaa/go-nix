package eval

import (
	"sync"
	"testing"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// TestWorkerHoldsItsOwnState is the milestone of phase D0: an evaluation runs
// on a worker handed to it, not on state the package keeps, so that there can
// eventually be more than one at a time.
//
// It does not run two at once — the symbol table, the per-node cache and the
// blackholing flag are still shared, and phases D1 to D3 are what make those
// safe. What it does assert is that nothing in the evaluator reaches past the
// worker it was given: a second worker draws its expressions from its own
// block, keeps its own stack depth, and leaves the package's worker alone.
func TestWorkerHoldsItsOwnState(t *testing.T) {
	pr, err := p.ParseString(`let f = x: x + 1; in f 41`)
	if err != nil {
		t.Fatal(err)
	}
	mainBefore := mainWorker.exprs

	other := &worker{}
	val, err := catching(func() NixValue { return delay(other, DefaultScope, pr).Eval(other) })
	if err != nil {
		t.Fatal(err)
	}
	if got := val.Int(); got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if other.depth != 0 {
		t.Errorf("worker left %d frames on its stack", other.depth)
	}
	if len(other.exprs) == 0 {
		t.Error("worker did not hand out expressions from its own block")
	}
	if !sameBlock(mainWorker.exprs, mainBefore) {
		t.Error("evaluation on another worker drew from the package's worker")
	}

	// A failure must unwind the worker it happened on, and only that one.
	badpr, err := p.ParseString(`let f = x: x + 1; in f (throw "boom")`)
	if err != nil {
		t.Fatal(err)
	}
	third := &worker{}
	if _, err := catching(func() NixValue {
		return delay(third, DefaultScope, badpr).Eval(third)
	}); err == nil {
		t.Fatal("expected the evaluation to fail")
	}
	if third.depth != 0 {
		t.Errorf("a failed evaluation left %d frames behind", third.depth)
	}
	if other.depth != 0 || mainWorker.depth != 0 {
		t.Error("a failure on one worker disturbed another")
	}
}

// sameBlock reports whether two views of a slab block are the same view.
func sameBlock(a, b []Expression) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// TestConcurrentEvaluationSharesTheFile is the milestone of phase D2: several
// workers evaluate the same syntax at once, sharing its static store and its
// literal values (which D1 made read-only) and the one symbol table (which D2
// made safe). It must pass under -race, and every worker must agree.
//
// The source pins both halves of interning: `"common"` is a literal used as a
// name, whose symbol the pass worked out in advance, and `"key${toString n}"`
// is a name no pass could know, interned as it is evaluated.
func TestConcurrentEvaluationSharesTheFile(t *testing.T) {
	pr, err := p.ParseString(`let
		mk = n: { "key${toString n}" = n; "common" = "same"; };
	in builtins.map mk [ 1 2 3 ]`)
	if err != nil {
		t.Fatal(err)
	}
	f := newFile(pr) // prepared once, shared by every worker

	ref, err := evalShared(f, pr.Result)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := evalShared(f, pr.Result)
			if err != nil {
				t.Errorf("evaluation failed: %v", err)
				return
			}
			if got != ref {
				t.Errorf("worker disagreed:\n got %s\nwant %s", got, ref)
			}
		}()
	}
	close(start)
	wg.Wait()
}

// evalShared evaluates root in the shared file f on a fresh worker, returning
// its full rendering.
func evalShared(f *file, root *p.Node) (string, error) {
	w := &worker{}
	scope := *DefaultScope
	scope.file = f
	x := w.newExpr()
	x.setThunk(&scope, root)
	val, err := catching(func() NixValue { return x.Eval(w) })
	if err != nil {
		return "", err
	}
	return catching(func() string { return val.Print(w, -1) })
}
