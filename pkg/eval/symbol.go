package eval

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// Sym is an interned name. It is int32 so that it packs into the tail of an
// Expression without widening it.
type Sym int32

// Symtab interns names, so that equal names are equal symbols and can be
// compared by identity. One table is shared by every evaluation, and workers
// evaluating in parallel intern the names they compute — an attribute name
// built at run time, a key read out of JSON.
//
// Two things in it are read differently often. The map from name to symbol is
// only touched by Intern, which is rare: the names the syntax gives are
// interned by the pass before any worker runs, so only computed names reach it
// during evaluation, and a mutex is enough. The slice from symbol back to name
// is read for every name rendered — attrNames alone renders one per attribute
// — so it is published with an atomic pointer and read without a lock: a
// reader that held the previous slice still finds every symbol that existed
// then, since Intern only ever appends.
type Symtab struct {
	mu   sync.Mutex
	syms map[string]Sym
	names atomic.Pointer[[]string]
}

func NewSymtab() *Symtab {
	st := &Symtab{syms: map[string]Sym{"": 0}}
	names := []string{""}
	st.names.Store(&names)
	return st
}

func (st *Symtab) Intern(name string) Sym {
	st.mu.Lock()
	defer st.mu.Unlock()
	if sym, ok := st.syms[name]; ok {
		return sym
	}
	names := append(*st.names.Load(), name)
	sym := Sym(len(names) - 1)
	st.syms[name] = sym
	st.names.Store(&names)
	return sym
}

func (st *Symtab) Name(sym Sym) string {
	return (*st.names.Load())[sym]
}

// TODO: Not capable of multiple (large?) files?
var globalSymtab = NewSymtab()

// Symbols the evaluator itself looks up.
var (
	symToString = Intern("__toString")
	symOutPath  = Intern("outPath")
	symRight    = Intern("right")
	symWrong    = Intern("wrong")
	symSuccess  = Intern("success")
	symValue    = Intern("value")
	symBuiltins = Intern("builtins")
)

func Intern(name string) Sym {
	return globalSymtab.Intern(name)
}

func (sym Sym) String() string {
	return globalSymtab.Name(sym)
}

// Sort syms for declarative printing
func SortSym(list []Sym) {
	slices.SortFunc(list, func(a, b Sym) int {
		return strings.Compare(a.String(), b.String())
	})
}
