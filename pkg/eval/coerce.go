package eval

import (
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
	switch val.Kind() {
	case KindString:
		return val.Str()
	case KindPath:
		return newString(val.Path().String())
	case KindSet:
		return val.Set().coerceToString(more)
	}
	if !more {
		throwf(ErrType, "cannot coerce %s to a string", anTypeName(val))
	}
	switch val.Kind() {
	case KindInt:
		return newString(printInt(val.Int()))
	case KindFloat:
		return newString(printFloat(val.Float()))
	case KindBool:
		// Nix renders true as "1" and false as the empty string.
		if val.Bool() {
			return newString("1")
		}
		return newString("")
	case KindNull:
		return newString("")
	case KindList:
		return coerceListToString(val.List(), more)
	}
	throwf(ErrType, "cannot coerce %s to a string", anTypeName(val))
	return nil
}

// coerceToString on a set uses __toString if present, else outPath, which is
// what makes a derivation usable inside a string.
func (s *AttrSet) coerceToString(more bool) *NixString {
	if x, ok := s.Get(symToString); ok {
		val := x.Eval()
		if !val.IsLambda() {
			throwf(ErrType, "value of the __toString attribute is %s while a function was expected",
				anTypeName(val))
		}
		return coerceToString(applyToValue(val.Lambda(), SetValue(s)).Eval(), more)
	}
	if x, ok := s.Get(symOutPath); ok {
		return coerceToString(x.Eval(), more)
	}
	throwf(ErrType, "cannot coerce a set to a string: it has neither a __toString nor an outPath attribute")
	return nil
}

// coerceListToString joins the coerced elements with spaces.
func coerceListToString(l NixList, more bool) *NixString {
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
