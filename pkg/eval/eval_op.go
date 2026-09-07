package eval

import (
	p "github.com/aleksanaa/go-nix/pkg/parser"
)

func (x *Expression) operand(i int) *Expression {
	return x.WithNode(x.Node.Nodes[i])
}

func (x *Expression) evalUnaryOp(nt p.NodeType) NixValue {
	switch nt {
	case p.OpNotNode:
		return !assertBool(x.operand(0).Eval())

	case p.OpNegateNode:
		switch v := x.operand(0).Eval().(type) {
		case NixInt:
			return -v
		case NixFloat:
			return -v
		default:
			throwf(ErrType, "value is %s while a number was expected", anTypeName(v))
		}

	case p.OpQuestionNode:
		// `e ? a.b` tests for an attribute path without forcing the value it
		// finds; a non-set anywhere along the path simply makes it false.
		val := x.operand(0).Eval()
		path := x.operand(1).evalAttrPath()
		for i, sym := range path {
			set, ok := val.(NixSet)
			if !ok {
				return False
			}
			y, found := set[sym]
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
	return nil
}

func (x *Expression) evalBinaryOp(nt p.NodeType) NixValue {
	switch nt {
	// `&&`, `||` and `->` are short-circuiting, so the right operand is only
	// forced when the left one does not already decide the answer.
	case p.OpAndNode:
		if !assertBool(x.operand(0).Eval()) {
			return NixBool(false)
		}
		return assertBool(x.operand(1).Eval())

	case p.OpOrNode:
		if assertBool(x.operand(0).Eval()) {
			return NixBool(true)
		}
		return assertBool(x.operand(1).Eval())

	case p.OpImplNode:
		if !assertBool(x.operand(0).Eval()) {
			return NixBool(true)
		}
		return assertBool(x.operand(1).Eval())
	}

	lhs, rhs := x.operand(0).Eval(), x.operand(1).Eval()
	switch nt {
	case p.OpAddNode:
		return Add(lhs, rhs)

	case p.OpConcatNode:
		return assertList(lhs).Concat(assertList(rhs))

	case p.OpUpdateNode:
		return assertSet(lhs).Update(assertSet(rhs))

	case p.OpEqNode:
		return NixBool(lhs.Compare(rhs))

	case p.OpLessNode:
		return NixBool(CompareOrder(lhs, rhs) < 0)

	case p.OpGreaterNode:
		return NixBool(CompareOrder(lhs, rhs) > 0)

	case p.OpLeqNode:
		return NixBool(CompareOrder(lhs, rhs) <= 0)

	case p.OpGeqNode:
		return NixBool(CompareOrder(lhs, rhs) >= 0)

	case p.OpNeqNode:
		return NixBool(!lhs.Compare(rhs))

	default:
		return Arith(lhs, rhs, nt)
	}
}
