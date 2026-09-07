package eval

// Scope is a chain of attribute sets an identifier is looked up in.
//
// `with` introduces a low-priority scope: names it provides are only used once
// the whole chain has been searched for ordinary (lexical) bindings, and a
// nearer `with` shadows a farther one.
type Scope struct {
	Binds   NixSet
	LowPrio bool
	Parent  *Scope
}

func (scope *Scope) Subscope(binds NixSet, lowPrio bool) *Scope {
	return &Scope{Binds: binds, LowPrio: lowPrio, Parent: scope}
}

// Lookup finds sym, searching lexical bindings first and `with` bindings only
// afterwards.
func (scope *Scope) Lookup(sym Sym) (*Expression, bool) {
	for _, lowPrio := range [2]bool{false, true} {
		for s := scope; s != nil; s = s.Parent {
			if s.LowPrio != lowPrio {
				continue
			}
			if x, ok := s.Binds[sym]; ok {
				return x, true
			}
		}
	}
	return nil, false
}
