package eval

import (
	"fmt"

	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// printInt renders an integer the way Nix does.
func printInt(i int64) string { return fmt.Sprintf("%d", i) }

// printFloat matches the %.6g Nix uses to display floats.
func printFloat(f float64) string { return fmt.Sprintf("%.6g", f) }

// NixNumber is what arithmetic works on. The operand types are known where
// arith is called, so this is one of the places a type parameter costs
// nothing: each instance is compiled for the numbers it actually adds.
type NixNumber interface{ int64 | float64 }

// arith applies an arithmetic operator to two numbers of the same type, and
// wraps the result as the value of that type.
func arith[T NixNumber](a, b T, op p.NodeType, wrap func(T) NixValue) NixValue {
	switch op {
	case p.OpAddNode:
		return wrap(a + b)
	case p.OpReduceNode:
		return wrap(a - b)
	case p.OpMultiplyNode:
		return wrap(a * b)
	case p.OpDivideNode:
		if b == 0 {
			throwf(ErrEval, "division by zero")
		}
		return wrap(a / b)
	}
	throwf(ErrEval, "unsupported arithmetic operator: %v", op)
	return NixValue{}
}

// Arith applies an arithmetic operator, promoting to float unless both
// operands are integers.
func Arith(a, b NixValue, op p.NodeType) NixValue {
	if a.kind == KindInt && b.kind == KindInt {
		return arith(a.Int(), b.Int(), op, Int)
	}
	if !a.IsNumber() || !b.IsNumber() {
		bad := a
		if a.IsNumber() {
			bad = b
		}
		throwf(ErrType, "value is %s while a number was expected", anTypeName(bad))
	}
	return arith(a.toFloat(), b.toFloat(), op, Float)
}

// Add implements `+`, which concatenates strings and paths as well as adding
// numbers.
func Add(a, b NixValue) NixValue {
	switch a.kind {
	case KindString:
		return StrValue(a.Str().Concat(assertString(b)))
	case KindPath:
		// path + string and path + path both yield a path.
		switch b.kind {
		case KindString:
			return PathValue(a.Path().Join(b.Str().Content))
		case KindPath:
			return PathValue(a.Path().Join(b.Path().String()))
		}
		throwf(ErrType, "value is %s while a string was expected", anTypeName(b))
	}
	return Arith(a, b, p.OpAddNode)
}
