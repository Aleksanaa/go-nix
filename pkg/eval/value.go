package eval

import "strings"

// NixValue is an evaluated Nix value.
type NixValue interface {
	// Print renders the value, expanding nested lists and sets while recurse
	// is positive and abbreviating them as "[ ... ]" or "{ ... }" beyond that.
	Print(recurse int) string
	// Compare implements Nix equality: structural for data, always false for
	// functions.
	Compare(val NixValue) bool
}

// TypeName is the name builtins.typeOf gives to a value.
func TypeName(val NixValue) string {
	switch val.(type) {
	case NixInt:
		return "int"
	case NixFloat:
		return "float"
	case NixBool:
		return "bool"
	case *NixNull:
		return "null"
	case NixList:
		return "list"
	case NixSet:
		return "set"
	case *NixPath:
		return "path"
	case *NixString:
		return "string"
	case NixLambda:
		return "lambda"
	case nil:
		return "unevaluated"
	}
	return "unknown"
}

// anTypeName names a value the way error messages spell it, as in
// "value is a list while a set was expected".
func anTypeName(val NixValue) string {
	switch val.(type) {
	case NixInt:
		return "an integer"
	case NixFloat:
		return "a float"
	case NixBool:
		return "a Boolean"
	case *NixNull:
		return "null"
	case NixList:
		return "a list"
	case NixSet:
		return "a set"
	case *NixPath:
		return "a path"
	case *NixString:
		return "a string"
	case NixLambda:
		return "a function"
	case nil:
		return "an unevaluated expression"
	}
	return "of an unknown type"
}

// assert returns val as a T, reporting a Nix type error if it is not one.
func assertType[T NixValue](val NixValue, expected string) T {
	if t, ok := val.(T); ok {
		return t
	}
	throwf(ErrType, "value is %s while %s was expected", anTypeName(val), expected)
	var zero T
	return zero
}

func assertBool(val NixValue) NixBool      { return assertType[NixBool](val, "a Boolean") }
func assertInt(val NixValue) NixInt        { return assertType[NixInt](val, "an integer") }
func assertList(val NixValue) NixList      { return assertType[NixList](val, "a list") }
func assertSet(val NixValue) NixSet        { return assertType[NixSet](val, "a set") }
func assertString(val NixValue) *NixString { return assertType[*NixString](val, "a string") }
func assertLambda(val NixValue) NixLambda  { return assertType[NixLambda](val, "a function") }

// NixNull is the null value. It is a pointer type so that a nil NixValue
// (an unevaluated or missing value) stays distinguishable from Nix null.
type NixNull struct{}

var Null = &NixNull{}

func (n *NixNull) Print(recurse int) string { return "null" }

func (n *NixNull) Compare(val NixValue) bool {
	_, ok := val.(*NixNull)
	return ok
}

type NixBool bool

const (
	True  = NixBool(true)
	False = NixBool(false)
)

func (b NixBool) Print(recurse int) string {
	if b {
		return "true"
	}
	return "false"
}

func (b NixBool) Compare(val NixValue) bool {
	b2, ok := val.(NixBool)
	return ok && b == b2
}

// CompareOrder orders two values as Nix's relational operators do: numbers
// numerically, strings and paths lexicographically, and lists element by
// element. It reports -1, 0 or 1, and raises a type error for values Nix
// refuses to order.
func CompareOrder(a, b NixValue) int {
	switch lhs := a.(type) {
	case NixInt, NixFloat:
		x, _ := toFloat(lhs)
		y, ok := toFloat(b)
		if !ok {
			break
		}
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0

	case *NixString:
		if rhs, ok := b.(*NixString); ok {
			return strings.Compare(lhs.Content, rhs.Content)
		}

	case *NixPath:
		if rhs, ok := b.(*NixPath); ok {
			return strings.Compare(lhs.String(), rhs.String())
		}

	case NixList:
		rhs, ok := b.(NixList)
		if !ok {
			break
		}
		// Lexicographic: the first differing element decides, and a prefix
		// sorts before the longer list.
		for i := 0; i < len(lhs) && i < len(rhs); i++ {
			if c := CompareOrder(lhs[i].Eval(), rhs[i].Eval()); c != 0 {
				return c
			}
		}
		return sign(len(lhs) - len(rhs))
	}
	throwf(ErrType, "cannot compare %s with %s", anTypeName(a), anTypeName(b))
	return 0
}
