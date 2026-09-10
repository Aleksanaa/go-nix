package eval

import (
	"fmt"
	"sync"
	"testing"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// TestSymtabConcurrent hammers one symbol table from many goroutines, each
// interning the same names, and checks that equal names are equal symbols and
// that names round-trip. It is the direct test of phase D2's lock; the
// evaluation-level test is TestConcurrentEvaluationSharesTheFile.
func TestSymtabConcurrent(t *testing.T) {
	st := NewSymtab()
	const kinds = 8
	var names [kinds]string
	for i := range names {
		names[i] = fmt.Sprintf("name-%d", i)
	}

	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				st.Intern(names[j%kinds])
			}
		}()
	}
	wg.Wait()

	want := map[string]Sym{}
	for _, name := range names {
		want[name] = st.Intern(name)
	}
	for _, name := range names {
		if got := st.Name(want[name]); got != name {
			t.Errorf("Name(%d) = %q, want %q", want[name], got, name)
		}
	}
}

// TestStringLiteralsArePreInterned pins the pre-interning half of phase D2: a
// string literal's symbol is worked out by the pass, so that using one as an
// attribute name at evaluation time is a read of a shared value, never a write
// to it.
func TestStringLiteralsArePreInterned(t *testing.T) {
	pr, err := p.ParseString(`{ "common" = 1; "key${1}" = 2; x = "value"; }`)
	if err != nil {
		t.Fatal(err)
	}
	f := newFile(pr)
	var check func(n *p.Node)
	check = func(n *p.Node) {
		if n.Type == p.StringNode || n.Type == p.IStringNode {
			// val is set only for a string with nothing interpolated; an
			// interpolated one has no literal value to pre-intern.
			if s := f.static.get(n.ID).val.Str(); s != nil && s.sym == 0 {
				t.Errorf("string literal %q not pre-interned", s.Content)
			}
		}
		for _, c := range n.Nodes {
			check(c)
		}
	}
	check(pr.Result)
}
