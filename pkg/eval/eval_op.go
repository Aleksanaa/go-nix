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
			w.throwf(ErrType, "value is %s while a number was expected", anTypeName(v))
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
	// forced at once; see forkWorthy.
	var lhs, rhs NixValue
	if w.budget >= 2 && forkWorthy(w, x, nt) {
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

// forkWorthy reports whether the right operand of x is a recursive call whose
// two halves are worth forcing on separate workers.
//
// What the operator is decides it. `++` and `//` put the join on the critical
// path — the result is the size of its operands, so however cheap the halves
// become the operator still copies both — and an operand that is not a
// recursive call does not divide the work, so handing it over costs more than
// doing it. Only when neither holds is the worker pool turned on, which is the
// side effect of goParallel; asking it of every operator up front is what made
// a fold-shaped workload pay the atomic claim for nothing.
func forkWorthy(w *worker, x *Expression, nt p.NodeType) bool {
	switch nt {
	case p.OpConcatNode, p.OpUpdateNode:
		return false
	}
	rhs := x.node().Nodes[1]
	if rhs.Type != p.ApplyNode || !x.scope().file.static.get(rhs.ID).recursive {
		return false
	}
	return goParallel()
}

// operandsForked evaluates both sides of an operator that forces both, on two
// workers where there is one to spare. forkWorthy has already decided the
// operand is worth it.
//
// This is the only fork point inside the evaluator rather than inside a
// builtin, and it is the one a recursion needs: `f (k - 1) + f (k - 1)` is a
// tree of independent work that no list-shaped builtin ever sees. What keeps
// it from forking all the way down to the leaves — where handing the work over
// costs far more than doing it — is the budget: a worker splits what it has
// with the side it gives away, so the forking stops a few levels in and the
// subtrees run whole.
//
// It is out of line because a function that starts a goroutine cannot be
// inlined, and evalBinaryOp is on the path of every operator in every
// evaluation.
func (x *Expression) operandsForked(w *worker) (lhs, rhs NixValue) {
	y := x.WithNode(w, x.node().Nodes[1])
	give := w.budget / 2
	w.budget -= give
	// The budget is given back however this returns. The wait for the fork is
	// not: it is called below rather than deferred, so that a failure unwinding
	// past here does not wait while still holding claims the fork may want. See
	// the note on waiting for a fork in pool.go.
	defer func() { w.budget += give }()

	done := make(chan struct{})
	end := forkBegin()
	go func() {
		defer close(done)
		defer end()
		cw := takeWorker()
		defer dropWorker(cw)
		cw.budget = give
		// A failure here is not this goroutine's to report: the value is
		// forced again below, in order, and fails there with its backtrace.
		catching(func() struct{} { y.Eval(cw); return struct{}{} })
	}()

	lhs = x.operand(w, 0)
	// Forced rather than taken from the channel: while the forked worker still
	// holds the claim this blocks behind it in the wait graph, where a cycle
	// between the two of us is seen and reported instead of hanging.
	rhs = y.Eval(w)
	// Nothing is left for it to claim now that y has a value, so this cannot
	// block on us, and it keeps the fork from outliving the operator.
	<-done
	return lhs, rhs
}
