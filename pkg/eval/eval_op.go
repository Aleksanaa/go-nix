package eval

import (
	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// operand evaluates the i-th child of the current node.
func (x *Expression) operand(i int) NixValue {
	return x.evalNode(x.node().Nodes[i])
}

func (x *Expression) evalUnaryOp(nt p.NodeType) NixValue {
	switch nt {
	case p.OpNotNode:
		return Bool(!assertBool(x.operand(0)))

	case p.OpNegateNode:
		switch v := x.operand(0); v.Kind() {
		case KindInt:
			return Int(-v.Int())
		case KindFloat:
			return Float(-v.Float())
		default:
			throwf(ErrType, "value is %s while a number was expected", anTypeName(v))
		}

	case p.OpQuestionNode:
		// `e ? a.b` tests for an attribute path without forcing the value it
		// finds; a non-set anywhere along the path simply makes it false.
		val := x.operand(0)
		path := x.scope().evalAttrPath(x.node().Nodes[1])
		for i, sym := range path {
			if val.Kind() != KindSet {
				return False
			}
			y, found := val.Set().Get(sym)
			if !found {
				return False
			}
			if i == len(path)-1 {
				// The last component is only tested for, never forced.
				break
			}
			val = y.Eval()
		}
		return True
	}
	throwf(ErrEval, "unsupported unary operator: %v", nt)
	return NixValue{}
}

func (x *Expression) evalBinaryOp(nt p.NodeType) NixValue {
	switch nt {
	// `&&`, `||` and `->` are short-circuiting, so the right operand is only
	// forced when the left one does not already decide the answer.
	case p.OpAndNode:
		if !assertBool(x.operand(0)) {
			return False
		}
		return Bool(assertBool(x.operand(1)))

	case p.OpOrNode:
		if assertBool(x.operand(0)) {
			return True
		}
		return Bool(assertBool(x.operand(1)))

	case p.OpImplNode:
		if !assertBool(x.operand(0)) {
			return True
		}
		return Bool(assertBool(x.operand(1)))
	}

	lhs, rhs := x.operand(0), x.operand(1)
	switch nt {
	case p.OpAddNode:
		return Add(lhs, rhs)

	case p.OpConcatNode:
		return ListValue(assertList(lhs).Concat(assertList(rhs)))

	case p.OpUpdateNode:
		return SetValue(assertSet(lhs).Update(assertSet(rhs)))

	case p.OpEqNode:
		return Bool(lhs.Compare(rhs))

	case p.OpLessNode:
		return Bool(CompareOrder(lhs, rhs) < 0)

	case p.OpGreaterNode:
		return Bool(CompareOrder(lhs, rhs) > 0)

	case p.OpLeqNode:
		return Bool(CompareOrder(lhs, rhs) <= 0)

	case p.OpGeqNode:
		return Bool(CompareOrder(lhs, rhs) >= 0)

	case p.OpNeqNode:
		return Bool(!lhs.Compare(rhs))

	default:
		return Arith(lhs, rhs, nt)
	}
}
