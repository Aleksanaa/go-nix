package eval

import (
	"sync"
	"testing"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// TestSharedStringInterning drives workers at different elements of one list,
// so that they force different thunks at the same time rather than queueing on
// a common root. Every element interns the same shared string, which is where
// NixString caches the symbol for its content.
func TestSharedStringInterning(t *testing.T) {
	evaluatingInParallel(t)
	pr, err := p.ParseString(`let
		name = "k${toString 1}";
		s = { k1 = 42; };
	in builtins.genList (i: builtins.getAttr name s) 64`)
	if err != nil {
		t.Fatal(err)
	}
	f := newFile(pr)
	scope := *DefaultScope
	scope.file = f
	root := newWorker().newExpr()
	root.setThunk(&scope, pr.Result)

	// The list itself is forced here; its elements are still thunks, and each
	// worker below forces all of them, so they meet on every one.
	w0 := newWorker()
	list, err := catching(func() NixList { return root.Eval(w0).List() })
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for k := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := newWorker()
			<-start
			// Each worker starts at a different element, so no two are
			// queueing behind the same thunk when they begin.
			for i := range list {
				x := list[(i+k*4)%len(list)]
				got, err := catching(func() int64 { return x.Eval(w).Int() })
				if err != nil {
					t.Errorf("%v", err)
					return
				}
				if got != 42 {
					t.Errorf("got %d, want 42", got)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
}
