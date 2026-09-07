package eval

import (
	"fmt"

	p "github.com/orivej/go-nix/pkg/parser"
)

type NixInt int64

func (i NixInt) Print(recurse int) string { return fmt.Sprintf("%d", i) }

func (i NixInt) Compare(val NixValue) bool {
	switch v := val.(type) {
	case NixInt:
		return i == v
	case NixFloat:
		return NixFloat(i).Compare(v)
	}
	return false
}

type NixFloat float64

// Print matches the %.6g Nix uses to display floats.
func (f NixFloat) Print(recurse int) string { return fmt.Sprintf("%.6g", f) }

func (f NixFloat) Compare(val NixValue) bool {
	switch v := val.(type) {
	case NixFloat:
		return f == v
	case NixInt:
		return f == NixFloat(v)
	}
	return false
}

type NixNumber interface {
	NixInt | NixFloat
}

func arith[T NixNumber](a, b T, op p.NodeType) NixValue {
	switch op {
	case p.OpAddNode:
		return NixValue(a + b)
	case p.OpReduceNode:
		return NixValue(a - b)
	case p.OpMultiplyNode:
		return NixValue(a * b)
	case p.OpDivideNode:
		if b == 0 {
			throwf(ErrEval, "division by zero")
		}
		return NixValue(a / b)
	}
	throwf(ErrEval, "unsupported arithmetic operator: %v", op)
	return nil
}

// Arith applies an arithmetic operator, promoting to float unless both
// operands are integers.
func Arith(a, b NixValue, op p.NodeType) NixValue {
	if i, ok := a.(NixInt); ok {
		if j, ok := b.(NixInt); ok {
			return arith(i, j, op)
		}
	}
	f, ok := toFloat(a)
	g, ok2 := toFloat(b)
	if !ok || !ok2 {
		bad := a
		if ok {
			bad = b
		}
		throwf(ErrType, "value is %s while a number was expected", anTypeName(bad))
	}
	return arith(f, g, op)
}

func toFloat(val NixValue) (NixFloat, bool) {
	switch v := val.(type) {
	case NixFloat:
		return v, true
	case NixInt:
		return NixFloat(v), true
	}
	return 0, false
}

// Add implements `+`, which concatenates strings and paths as well as adding
// numbers.
func Add(a, b NixValue) NixValue {
	switch lhs := a.(type) {
	case *NixString:
		return lhs.Concat(assertString(b))
	case *NixPath:
		// path + string and path + path both yield a path.
		switch rhs := b.(type) {
		case *NixString:
			return lhs.Join(rhs.Content)
		case *NixPath:
			return lhs.Join(rhs.String())
		}
		throwf(ErrType, "value is %s while a string was expected", anTypeName(b))
	}
	return Arith(a, b, p.OpAddNode)
}
