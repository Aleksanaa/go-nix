package eval

import (
	p "github.com/aleksanaa/go-nix/pkg/parser"
)

// operand evaluates the i-th child of the current node.
func (x *Expression) operand(w *worker, i int) NixValue {
	return x.evalNode(w, x.node().Nodes[i])
}

func (x *Expression) evalUnaryOp(w *worker, nt p.NodeType) NixValue {
	switch nt {
	case p.OpNotNode:
		return Bool(!assertBool(w, x.operand(w, 0)))

	case p.OpNegateNode:
		switch v := x.operand(w, 0); v.Kind() {
		case KindInt:
			return Int(-v.Int())
		case KindFloat:
			return Float(-v.Float())
		default:
			w.throwf(ErrType, "value is %s while a number was expected", anTypeName(w, v))
		}

	case p.OpQuestionNode:
		// `e ? a.b` tests for an attribute path without forcing the value it
		// finds; a non-set anywhere along the path simply makes it false.
		val := x.operand(w, 0)
		path := x.scope().evalAttrPath(w, x.node().Nodes[1])
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
			val = y.Eval(w)
		}
		return True
	}
	w.throwf(ErrEval, "unsupported unary operator: %v", nt)
	return NixValue{}
}

func (x *Expression) evalBinaryOp(w *worker, nt p.NodeType) NixValue {
	switch nt {
	// `&&`, `||` and `->` are short-circuiting, so the right operand is only
	// forced when the left one does not already decide the answer.
	case p.OpAndNode:
		if !assertBool(w, x.operand(w, 0)) {
			return False
		}
		return Bool(assertBool(w, x.operand(w, 1)))

	case p.OpOrNode:
		if assertBool(w, x.operand(w, 0)) {
			return True
		}
		return Bool(assertBool(w, x.operand(w, 1)))

	case p.OpImplNode:
		if !assertBool(w, x.operand(w, 0)) {
			return True
		}
		return Bool(assertBool(w, x.operand(w, 1)))
	}

	// Both sides are forced, so where there is a worker to spare they can be
	// forced at once; see operandsForked.
	var lhs, rhs NixValue
	if forkOps && w.budget >= 2 && goParallel() {
		lhs, rhs = x.operandsForked(w)
	} else {
		lhs, rhs = x.operand(w, 0), x.operand(w, 1)
	}
	switch nt {
	case p.OpAddNode:
		return Add(w, lhs, rhs)

	case p.OpConcatNode:
		return ListValue(assertList(w, lhs).Concat(assertList(w, rhs)))

	case p.OpUpdateNode:
		return SetValue(assertSet(w, lhs).Update(assertSet(w, rhs)))

	case p.OpEqNode:
		return Bool(lhs.Compare(w, rhs))

	case p.OpLessNode:
		return Bool(CompareOrder(w, lhs, rhs) < 0)

	case p.OpGreaterNode:
		return Bool(CompareOrder(w, lhs, rhs) > 0)

	case p.OpLeqNode:
		return Bool(CompareOrder(w, lhs, rhs) <= 0)

	case p.OpGeqNode:
		return Bool(CompareOrder(w, lhs, rhs) >= 0)

	case p.OpNeqNode:
		return Bool(!lhs.Compare(w, rhs))

	default:
		return Arith(w, lhs, rhs, nt)
	}
}

// operands evaluates both sides of an operator that forces both, on two
// workers where there is one to spare.
//
// This is the only fork point inside the evaluator rather than inside a
// builtin, and it is the one a recursion needs: `f (k - 1) + f (k - 1)` is a
// tree of independent work that no list-shaped builtin ever sees. What keeps
// it from forking all the way down to the leaves — where handing the work over
// costs far more than doing it — is the budget: a worker splits what it has
// with the side it gives away, so the forking stops a few levels in and the
// subtrees run whole.
// operandsForked is operands where a worker is to be spared. It is out of
// line because a function that starts a goroutine cannot be inlined, and
// operands is on the path of every operator in every evaluation.
func (x *Expression) operandsForked(w *worker) (lhs, rhs NixValue) {
	rhsNode := x.node().Nodes[1]
	// The node type is in hand and settles most operands — a name, a literal —
	// without reaching for what the pass worked out about them.
	if rhsNode.Type != p.ApplyNode || !x.scope().file.static.get(rhsNode.ID).recursive {
		return x.operand(w, 0), x.operand(w, 1)
	}
	y := x.WithNode(w, rhsNode)
	give := w.budget / 2
	w.budget -= give

	done := make(chan struct{})
	go func() {
		defer close(done)
		cw := takeWorker()
		defer dropWorker(cw)
		cw.budget = give
		// A failure here is not this goroutine's to report: the value is
		// forced again below, in order, and fails there with its backtrace.
		catching(func() struct{} { y.Eval(cw); return struct{}{} })
	}()
	// Waited for even when the left side fails, so that no work outlives the
	// evaluation that asked for it.
	defer func() {
		<-done
		w.budget += give
	}()

	// Receiving twice from a closed channel is harmless, so the defer above
	// simply finds the wait already done.
	lhs = x.operand(w, 0)
	<-done
	return lhs, y.Eval(w)
}
