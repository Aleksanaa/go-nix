package eval

import (
	"slices"
	"strings"
)

// Sym is an interned name. It is int32 so that it packs into the tail of an
// Expression without widening it.
type Sym int32

// Symtab interns names, so that equal names are equal symbols and can be
// compared by identity. One table is shared by every evaluation: the names the
// syntax gives are interned by the pass before evaluation begins, and only a
// name computed at run time — an attribute name built out of an
// interpolation, a key read out of JSON — reaches it during one.
type Symtab struct {
	syms  map[string]Sym
	names []string
}

func NewSymtab() *Symtab {
	return &Symtab{syms: map[string]Sym{"": 0}, names: []string{""}}
}

func (st *Symtab) Intern(name string) Sym {
	if sym, ok := st.syms[name]; ok {
		return sym
	}
	st.names = append(st.names, name)
	sym := Sym(len(st.names) - 1)
	st.syms[name] = sym
	return sym
}

func (st *Symtab) Name(sym Sym) string { return st.names[sym] }

// TODO: Not capable of multiple (large?) files?
var globalSymtab = NewSymtab()

// Symbols the evaluator itself looks up.
var (
	symToString        = Intern("__toString")
	symOutPath         = Intern("outPath")
	symDrvPath         = Intern("drvPath")
	symRight           = Intern("right")
	symWrong           = Intern("wrong")
	symSuccess         = Intern("success")
	symValue           = Intern("value")
	symName            = Intern("name")
	symBuiltins        = Intern("builtins")
	symType            = Intern("type")
	symOutputName      = Intern("outputName")
	symBuilder         = Intern("builder")
	symSystem          = Intern("system")
	symArgs            = Intern("args")
	symOutputs         = Intern("outputs")
	symIgnoreNulls     = Intern("__ignoreNulls")
	symStructuredAttrs = Intern("__structuredAttrs")
	symAll             = Intern("all")
	symDrvAttrs        = Intern("drvAttrs")
	symPath            = Intern("path")
	symFilter          = Intern("filter")
	symAllOutputs      = Intern("allOutputs")
	symFile            = Intern("file")
	symLine            = Intern("line")
	symColumn          = Intern("column")
	symStartSet        = Intern("startSet")
	symOperator        = Intern("operator")
	symKey             = Intern("key")
	symHash            = Intern("hash")
	symHashAlgo        = Intern("hashAlgo")
	symToHashFormat    = Intern("toHashFormat")
	symFunctor         = Intern("__functor")
	symOutputHash      = Intern("outputHash")
	symOutputHashAlgo  = Intern("outputHashAlgo")
	symOutputHashMode  = Intern("outputHashMode")
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
