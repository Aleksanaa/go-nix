package eval

import "strings"

// NixList is a list of lazily evaluated elements.
type NixList []*Expression

func (l NixList) Print(recurse int) string {
	if recurse == 0 {
		return "[ ... ]"
	}
	parts := make([]string, 0, len(l)+2)
	parts = append(parts, "[")
	for _, x := range l {
		parts = append(parts, x.Eval().Print(recurse-1))
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

func (l NixList) Compare(other NixList) bool {
	if len(l) != len(other) {
		return false
	}
	for i, x := range l {
		if !x.Eval().Compare(other[i].Eval()) {
			return false
		}
	}
	return true
}
