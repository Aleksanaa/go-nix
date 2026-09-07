package eval

import (
	"fmt"
	"strings"
)

// NixSet is an attribute set mapping names to lazily evaluated values.
type NixSet map[Sym]*Expression

// Keys returns the attribute names in the order Nix presents them:
// alphabetically, so that printing and builtins.attrNames are deterministic.
func (s NixSet) Keys() []Sym {
	keys := make([]Sym, 0, len(s))
	for sym := range s {
		keys = append(keys, sym)
	}
	SortSym(keys)
	return keys
}

func (s NixSet) Print(recurse int) string {
	if recurse == 0 {
		return "{ ... }"
	}
	parts := make([]string, 0, len(s)+2)
	parts = append(parts, "{")
	for _, key := range s.Keys() {
		parts = append(parts, fmt.Sprintf("%s = %s;", key, s[key].Eval().Print(recurse-1)))
	}
	return strings.Join(append(parts, "}"), " ")
}

// Bind1 adds a single attribute, rejecting a redefinition.
func (s NixSet) Bind1(sym Sym, x *Expression) {
	if _, ok := s[sym]; ok {
		throwf(ErrEval, "attribute '%s' already defined", sym)
	}
	s[sym] = x
}

// Bind adds an attribute under a path, creating the intermediate sets that
// `a.b.c = v;` implies.
func (s NixSet) Bind(syms []Sym, x *Expression) {
	last := len(syms) - 1
	for i, sym := range syms[:last] {
		y, ok := s[sym]
		if !ok {
			sub := NixSet{}
			s[sym] = value(sub)
			s = sub
			continue
		}
		// The intermediate set must be one this binding group created; merging
		// into an attribute that is already a value would not be lazy.
		sub, ok := y.Value.(NixSet)
		if !ok {
			throwf(ErrEval, "attribute '%s' already defined", strings.Join(symNames(syms[:i+1]), "."))
		}
		s = sub
	}
	s.Bind1(syms[last], x)
}

// Update returns `s // other`.
func (s NixSet) Update(other NixSet) NixSet {
	result := make(NixSet, len(s)+len(other))
	for sym, x := range s {
		result[sym] = x
	}
	for sym, x := range other {
		result[sym] = x
	}
	return result
}

func (s NixSet) Compare(val NixValue) bool {
	other, ok := val.(NixSet)
	if !ok || len(s) != len(other) {
		return false
	}
	for sym, x := range s {
		y, ok := other[sym]
		if !ok || !x.Eval().Compare(y.Eval()) {
			return false
		}
	}
	return true
}

func symNames(syms []Sym) []string {
	names := make([]string, len(syms))
	for i, sym := range syms {
		names[i] = sym.String()
	}
	return names
}
