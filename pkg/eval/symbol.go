package eval

import (
	"slices"
	"strings"
)

// Sym is an interned name. It is int32 so that it packs into the tail of an
// Expression without widening it.
type Sym int32

type Symtab struct {
	names []string
	syms  map[string]Sym
}

func NewSymtab() *Symtab {
	return &Symtab{names: []string{""}, syms: map[string]Sym{"": 0}}
}

func (st *Symtab) Intern(name string) Sym {
	if sym, ok := st.syms[name]; ok {
		return sym
	}
	sym := Sym(len(st.names))
	st.names = append(st.names, name)
	st.syms[name] = sym
	return sym
}

func (st *Symtab) Name(sym Sym) string {
	return st.names[sym]
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
