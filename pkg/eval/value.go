package eval

import (
	"strings"
	"unsafe"
)

// Kind says what a value is. The zero kind is the absence of one, which is
// what a thunk that has not been forced holds.
type Kind uint8

const (
	KindNone Kind = iota
	KindNull
	KindBool
	KindInt
	KindFloat
	KindString
	KindPath
	KindList
	KindSet
	KindLambda  // a function written in Nix
	KindPrimop  // a builtin
	KindPartial // a builtin waiting for more arguments
)

// NixValue is an evaluated Nix value: what kind it is, a word for the payload
// of the kinds that fit in one, and a pointer for the kinds that do not.
//
// Nix uses the same representation, and for the same reason: with values held
// in an interface every integer has to be boxed on the heap to be passed
// around, which costs an allocation per arithmetic operation and a pointer for
// the collector to chase afterwards. Here a number lives in the value itself.
//
// The zero value is no value at all, which is how an unevaluated expression
// reads: see Expression.
type NixValue struct {
	// ptr is the value for the kinds that are held by pointer, and the first
	// element of the array for a list.
	ptr unsafe.Pointer
	// num is the payload of the kinds that fit in a word: an integer, the bits
	// of a float, a Boolean, and the length of a list.
	num  int64
	kind Kind
}

// The values that need no payload at all.
var (
	Null  = NixValue{kind: KindNull}
	True  = NixValue{kind: KindBool, num: 1}
	False = NixValue{kind: KindBool}
)

// Kind is what the value is.
func (v NixValue) Kind() Kind { return v.kind }

// IsNone reports whether this is the absence of a value rather than a value.
func (v NixValue) IsNone() bool { return v.kind == KindNone }

// Int makes an integer value.
func Int(i int64) NixValue { return NixValue{kind: KindInt, num: i} }

// Float makes a floating point value.
func Float(f float64) NixValue {
	return NixValue{kind: KindFloat, num: int64(*(*uint64)(unsafe.Pointer(&f)))}
}

// Bool makes a Boolean value.
func Bool(b bool) NixValue {
	if b {
		return True
	}
	return False
}

// StrValue wraps a string with its context as a value.
func StrValue(s *NixString) NixValue {
	return NixValue{kind: KindString, ptr: unsafe.Pointer(s)}
}

// String makes a plain string value.
func String(s string) NixValue { return StrValue(newString(s)) }

// PathValue makes a path value.
func PathValue(p *NixPath) NixValue { return NixValue{kind: KindPath, ptr: unsafe.Pointer(p)} }

// SetValue makes a set value.
func SetValue(s NixSet) NixValue { return NixValue{kind: KindSet, ptr: unsafe.Pointer(s)} }

// ListValue makes a list value. The list is held as its first element and its
// length, which is what keeps a value one word wider than a pointer.
func ListValue(l NixList) NixValue {
	if len(l) == 0 {
		return NixValue{kind: KindList}
	}
	return NixValue{kind: KindList, ptr: unsafe.Pointer(&l[0]), num: int64(len(l))}
}

// LambdaValue makes a value of anything that can be applied.
func LambdaValue(f NixLambda) NixValue {
	switch fn := f.(type) {
	case *NixExprLambda:
		return NixValue{kind: KindLambda, ptr: unsafe.Pointer(fn)}
	case *NixPrimop:
		return NixValue{kind: KindPrimop, ptr: unsafe.Pointer(fn)}
	case *NixPartialPrimop:
		return NixValue{kind: KindPartial, ptr: unsafe.Pointer(fn)}
	}
	throwf(ErrEval, "unsupported function value")
	return NixValue{}
}

// Int returns the integer this holds, which the caller has checked the kind of.
func (v NixValue) Int() int64 { return v.num }

// Float returns the float this holds.
func (v NixValue) Float() float64 { return *(*float64)(unsafe.Pointer(&v.num)) }

// Bool returns the Boolean this holds.
func (v NixValue) Bool() bool { return v.num != 0 }

// Str returns the string this holds.
func (v NixValue) Str() *NixString { return (*NixString)(v.ptr) }

// Path returns the path this holds.
func (v NixValue) Path() *NixPath { return (*NixPath)(v.ptr) }

// Set returns the attribute set this holds.
func (v NixValue) Set() NixSet { return (*AttrSet)(v.ptr) }

// List returns the list this holds, rebuilt from its first element and length.
func (v NixValue) List() NixList {
	if v.num == 0 {
		return nil
	}
	return unsafe.Slice((**Expression)(v.ptr), v.num)
}

// Lambda returns the function this holds.
func (v NixValue) Lambda() NixLambda {
	switch v.kind {
	case KindLambda:
		return (*NixExprLambda)(v.ptr)
	case KindPrimop:
		return (*NixPrimop)(v.ptr)
	case KindPartial:
		return (*NixPartialPrimop)(v.ptr)
	}
	return nil
}

// IsLambda reports whether the value can be applied.
func (v NixValue) IsLambda() bool {
	return v.kind == KindLambda || v.kind == KindPrimop || v.kind == KindPartial
}

// IsNumber reports whether the value is an integer or a float.
func (v NixValue) IsNumber() bool { return v.kind == KindInt || v.kind == KindFloat }

// Print renders the value, expanding nested lists and sets while recurse is
// positive and abbreviating them as "[ ... ]" or "{ ... }" beyond that.
func (v NixValue) Print(recurse int) string {
	switch v.kind {
	case KindNone:
		return "«unevaluated»"
	case KindNull:
		return "null"
	case KindBool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case KindInt:
		return printInt(v.Int())
	case KindFloat:
		return printFloat(v.Float())
	case KindString:
		return v.Str().Print()
	case KindPath:
		return v.Path().String()
	case KindList:
		return v.List().Print(recurse)
	case KindSet:
		return v.Set().Print(recurse)
	case KindLambda:
		return "«lambda»"
	case KindPrimop, KindPartial:
		return v.Lambda().(interface{ printOp() string }).printOp()
	}
	return "«unknown»"
}

// Compare implements Nix equality: structural for data, always false for
// functions.
func (v NixValue) Compare(other NixValue) bool {
	switch v.kind {
	case KindNull:
		return other.kind == KindNull
	case KindBool:
		return other.kind == KindBool && v.num == other.num
	case KindInt:
		switch other.kind {
		case KindInt:
			return v.num == other.num
		case KindFloat:
			return float64(v.Int()) == other.Float()
		}
		return false
	case KindFloat:
		switch other.kind {
		case KindFloat:
			return v.Float() == other.Float()
		case KindInt:
			return v.Float() == float64(other.Int())
		}
		return false
	case KindString:
		return other.kind == KindString && v.Str().Content == other.Str().Content
	case KindPath:
		return other.kind == KindPath && v.Path().String() == other.Path().String()
	case KindList:
		return other.kind == KindList && v.List().Compare(other.List())
	case KindSet:
		return other.kind == KindSet && v.Set().Compare(other.Set())
	}
	// Functions never compare equal, nor does an absent value.
	return false
}

// TypeName is the name builtins.typeOf gives to a value.
func TypeName(val NixValue) string {
	switch val.kind {
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindBool:
		return "bool"
	case KindNull:
		return "null"
	case KindList:
		return "list"
	case KindSet:
		return "set"
	case KindPath:
		return "path"
	case KindString:
		return "string"
	case KindLambda, KindPrimop, KindPartial:
		return "lambda"
	case KindNone:
		return "unevaluated"
	}
	return "unknown"
}

// anTypeName names a value the way error messages spell it, as in
// "value is a list while a set was expected".
func anTypeName(val NixValue) string {
	switch val.kind {
	case KindInt:
		return "an integer"
	case KindFloat:
		return "a float"
	case KindBool:
		return "a Boolean"
	case KindNull:
		return "null"
	case KindList:
		return "a list"
	case KindSet:
		return "a set"
	case KindPath:
		return "a path"
	case KindString:
		return "a string"
	case KindLambda, KindPrimop, KindPartial:
		return "a function"
	case KindNone:
		return "an unevaluated expression"
	}
	return "of an unknown type"
}

// assertKind returns val when it is of the kind expected, and reports a Nix
// type error when it is not.
func assertKind(val NixValue, kind Kind, expected string) NixValue {
	if val.kind != kind {
		throwf(ErrType, "value is %s while %s was expected", anTypeName(val), expected)
	}
	return val
}

func assertBool(val NixValue) bool         { return assertKind(val, KindBool, "a Boolean").Bool() }
func assertInt(val NixValue) int64         { return assertKind(val, KindInt, "an integer").Int() }
func assertList(val NixValue) NixList      { return assertKind(val, KindList, "a list").List() }
func assertSet(val NixValue) NixSet        { return assertKind(val, KindSet, "a set").Set() }
func assertString(val NixValue) *NixString { return assertKind(val, KindString, "a string").Str() }

func assertLambda(val NixValue) NixLambda {
	if !val.IsLambda() {
		throwf(ErrType, "value is %s while a function was expected", anTypeName(val))
	}
	return val.Lambda()
}

// CompareOrder orders two values as Nix's relational operators do: numbers
// numerically, strings and paths lexicographically, and lists element by
// element. It reports -1, 0 or 1, and raises a type error for values Nix
// refuses to order.
func CompareOrder(a, b NixValue) int {
	switch {
	case a.IsNumber() && b.IsNumber():
		x, y := a.toFloat(), b.toFloat()
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0

	case a.kind == KindString && b.kind == KindString:
		return strings.Compare(a.Str().Content, b.Str().Content)

	case a.kind == KindPath && b.kind == KindPath:
		return strings.Compare(a.Path().String(), b.Path().String())

	case a.kind == KindList && b.kind == KindList:
		// Lexicographic: the first differing element decides, and a prefix
		// sorts before the longer list.
		lhs, rhs := a.List(), b.List()
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

// toFloat is the value as a float, for a value already known to be a number.
func (v NixValue) toFloat() float64 {
	if v.kind == KindInt {
		return float64(v.Int())
	}
	return v.Float()
}

// AsSet returns the set this value holds, and whether it is one. It is for
// callers outside the package, which cannot switch on the kind themselves.
func (v NixValue) AsSet() (NixSet, bool) {
	if v.kind != KindSet {
		return nil, false
	}
	return v.Set(), true
}

// AsPrimop returns the builtin this value holds, and whether it is one.
func (v NixValue) AsPrimop() (*NixPrimop, bool) {
	if v.kind != KindPrimop {
		return nil, false
	}
	return (*NixPrimop)(v.ptr), true
}
