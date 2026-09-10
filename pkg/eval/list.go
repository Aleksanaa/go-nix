package eval

import (
	"strings"
	"unsafe"
)

// NixList is a list of lazily evaluated elements.
type NixList []*Expression

func (l NixList) Print(w *worker, recurse int) string {
	if recurse == 0 {
		return "[ ... ]"
	}
	if w.seen != nil && len(l) > 0 {
		p := unsafe.Pointer(&l[0])
		if w.seen[p] {
			return "«repeated»"
		}
		w.seen[p] = true
	}
	parts := make([]string, 0, len(l)+2)
	parts = append(parts, "[")
	for _, x := range l {
		parts = append(parts, x.Eval(w).Print(w, recurse-1))
	}
	return strings.Join(append(parts, "]"), " ")
}

// Concat returns the concatenation of two lists. It copies rather than
// appending in place, since the backing array of the left operand may be
// shared with the list value it came from.
func (l NixList) Concat(other NixList) NixList {
	result := make(NixList, 0, len(l)+len(other))
	result = append(result, l...)
	return append(result, other...)
}

func (l NixList) Compare(w *worker, other NixList) bool {
	if len(l) != len(other) {
		return false
	}
	for i, x := range l {
		if !x.Eval(w).Compare(w, other[i].Eval(w)) {
			return false
		}
	}
	return true
}
