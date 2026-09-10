package eval

import (
	"fmt"
	"strings"
)

// CoerceToString converts a value to a string the way interpolation and
// attribute names do: only strings, paths and sets with an outPath or a
// __toString attribute are accepted.
func CoerceToString(val NixValue) *NixString {
	return coerceToString(val, false)
}

// ToString converts a value to a string the way builtins.toString does, which
// additionally accepts numbers, Booleans, null and lists.
func ToString(val NixValue) *NixString {
	return coerceToString(val, true)
}

func coerceToString(val NixValue, more bool) *NixString {
	switch v := val.(type) {
	case *NixString:
		return v
	case *NixPath:
		return String(v.String())
	case NixSet:
		return v.coerceToString(more)
	}
	if !more {
		throwf(ErrType, "cannot coerce %s to a string", anTypeName(val))
	}
	switch v := val.(type) {
	case NixInt:
		return String(fmt.Sprintf("%d", v))
	case NixFloat:
		return String(fmt.Sprintf("%.6g", v))
	case NixBool:
		// Nix renders true as "1" and false as the empty string.
		if v {
			return String("1")
		}
		return String("")
	case *NixNull:
		return String("")
	case NixList:
		return v.coerceToString(more)
	}
	throwf(ErrType, "cannot coerce %s to a string", anTypeName(val))
	return nil
}

// coerceToString on a set uses __toString if present, else outPath, which is
// what makes a derivation usable inside a string.
func (s NixSet) coerceToString(more bool) *NixString {
	if x, ok := s.Get(symToString); ok {
		fn, ok := x.Eval().(NixLambda)
		if !ok {
			throwf(ErrType, "value of the __toString attribute is %s while a function was expected",
				anTypeName(x.Value))
		}
		return coerceToString(fn.Apply(value(s)).Eval(), more)
	}
	if x, ok := s.Get(symOutPath); ok {
		return coerceToString(x.Eval(), more)
	}
	throwf(ErrType, "cannot coerce a set to a string: it has neither a __toString nor an outPath attribute")
	return nil
}

// coerceToString on a list joins the coerced elements with spaces.
func (l NixList) coerceToString(more bool) *NixString {
	result := &NixString{}
	parts := make([]string, len(l))
	for i, x := range l {
		str := coerceToString(x.Eval(), more)
		parts[i] = str.Content
		result.absorb(str)
	}
	result.Content = strings.Join(parts, " ")
	return result
}
