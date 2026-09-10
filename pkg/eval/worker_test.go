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

	other := newWorker()
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
	third := newWorker()
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
	evaluatingInParallel(t)
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
	w := newWorker()
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

// TestConcurrentForceSharesThunks is the milestone of phase D3: several
// workers force the *same* thunks, not merely the same syntax.
//
// That is the difference that matters, and the one the D2 test does not make:
// there, each worker built its own expressions over a shared parse, so no two
// ever met on a thunk. Here they are handed one tree and race to force it, so
// every claim, every wait for another worker's value, and every read of a
// value another worker wrote is exercised — which is what -race has to see.
func TestConcurrentForceSharesThunks(t *testing.T) {
	evaluatingInParallel(t)
	pr, err := p.ParseString(`let
		slow = n: if n == 0 then 0 else slow (n - 1) + 1;
		shared = slow 400;
	in builtins.genList (i: { a = shared + i; b = "n${toString shared}"; }) 24`)
	if err != nil {
		t.Fatal(err)
	}
	f := newFile(pr)
	scope := *DefaultScope
	scope.file = f

	// The expected answers come from a tree of their own, so that the one the
	// workers race on is still unforced when they start. Forcing that one here
	// would leave them nothing to contend for.
	tree := func() *Expression {
		x := newWorker().newExpr()
		x.setThunk(&scope, pr.Result)
		return x
	}
	render := func(x *Expression) ([]string, error) {
		return catching2(func() []string {
			w := newWorker()
			var out []string
			for _, el := range x.Eval(w).List() {
				out = append(out, el.Eval(w).Print(w, -1))
			}
			return out
		})
	}
	ref, err := render(tree())
	if err != nil {
		t.Fatal(err)
	}

	// The list is forced here; its elements are still thunks. Each worker
	// below forces all of them but starts somewhere different, so they are on
	// different thunks at the same time rather than queueing behind one root —
	// which is what makes them meet on a claim.
	w0 := newWorker()
	list, err := catching(func() NixList { return tree().Eval(w0).List() })
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for k := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := newWorker()
			for i := range list {
				x := list[(i+k*3)%len(list)]
				got, err := catching(func() string { return x.Eval(w).Print(w, -1) })
				if err != nil {
					t.Errorf("evaluation failed: %v", err)
					return
				}
				if want := ref[(i+k*3)%len(list)]; got != want {
					t.Errorf("worker disagreed:\n got %s\nwant %s", got, want)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
}

// evaluatingInParallel turns on the mode that makes a thunk's claim atomic,
// and turns it off again. A test that runs workers at once must set it: with
// one worker the claim is touched plainly, which is what keeps the ordinary
// evaluation as fast as it was before there were workers at all.
func evaluatingInParallel(t *testing.T) {
	t.Helper()
	was := parallel
	parallel = true
	t.Cleanup(func() { parallel = was })
}

// catching2 is catching for a slice, which its type parameter cannot infer
// from a bare call in a test.
func catching2(f func() []string) ([]string, error) { return catching(f) }
