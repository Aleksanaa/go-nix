package eval

import "sort"

func bAll(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	for _, x := range assertList(w, args[1].Eval(w)) {
		if !assertBool(w, f.Apply(w, x).Eval(w)) {
			return False
		}
	}
	return True
}

func bAny(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	for _, x := range assertList(w, args[1].Eval(w)) {
		if assertBool(w, f.Apply(w, x).Eval(w)) {
			return True
		}
	}
	return False
}

func bConcatLists(w *worker, args ...*Expression) NixValue {
	lists := assertList(w, args[0].Eval(w))
	result := make(NixList, 0, len(lists))
	for _, x := range lists {
		result = append(result, assertList(w, x.Eval(w))...)
	}
	return ListValue(result)
}

func bElem(w *worker, args ...*Expression) NixValue {
	val := args[0].Eval(w)
	for _, x := range assertList(w, args[1].Eval(w)) {
		if val.Compare(w, x.Eval(w)) {
			return True
		}
	}
	return False
}

func bElemAt(w *worker, args ...*Expression) NixValue {
	list := assertList(w, args[0].Eval(w))
	i := int64(assertInt(w, args[1].Eval(w)))
	if i < 0 || i >= int64(len(list)) {
		w.throwf(ErrEval, "list index %d is out of bounds, the list has %d elements", i, len(list))
	}
	return list[i].Eval(w)
}

func bFilter(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	list := assertList(w, args[1].Eval(w))
	// The predicate is asked about every element, so the questions can be put
	// in advance and answered across the pool. Only in parallel: building them
	// all up front costs an expression each, which is what the loop below
	// avoids when it is the only thing running.
	var asked []*Expression
	if len(list) >= forkMinItems && goParallel() {
		asked = make([]*Expression, len(list))
		for i, x := range list {
			asked[i] = f.Apply(w, x)
		}
		forceAll(w, asked)
	}
	result := make(NixList, 0, len(list))
	for i, x := range list {
		var y *Expression
		if asked != nil {
			y = asked[i]
		} else {
			y = f.Apply(w, x)
		}
		if assertBool(w, y.Eval(w)) {
			result = append(result, x)
		}
	}
	return ListValue(result)
}

// bFoldl is builtins.foldl', which forces the accumulator at every step.
func bFoldl(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	acc := args[1]
	for _, x := range assertList(w, args[2].Eval(w)) {
		// The step's own expression carries the accumulator into the next
		// round: forcing it here is what makes the fold strict, and it has
		// memoized the value, so nothing else needs to hold it.
		acc = apply2(w, f, acc, x)
		acc.Eval(w)
	}
	return acc.Eval(w)
}

func bGenList(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	n := int64(assertInt(w, args[1].Eval(w)))
	if n < 0 {
		w.throwf(ErrEval, "cannot create a list of %d elements", n)
	}
	result := make(NixList, n)
	// The index an element is applied to is a value rather than a computation,
	// so the expressions holding them are made in one block: they are all of
	// the same age and have the same lifetime as the list itself.
	idx := make([]Expression, n)
	for i := range result {
		idx[i].setValue(Int(int64(i)))
		result[i] = f.Apply(w, &idx[i])
	}
	return ListValue(result)
}

func bHead(w *worker, args ...*Expression) NixValue {
	list := assertList(w, args[0].Eval(w))
	if len(list) == 0 {
		w.throwf(ErrEval, "list index 0 is out of bounds, the list is empty")
	}
	return list[0].Eval(w)
}

func bLength(w *worker, args ...*Expression) NixValue {
	return Int(int64(len(assertList(w, args[0].Eval(w)))))
}

func bMap(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	list := assertList(w, args[1].Eval(w))
	result := make(NixList, len(list))
	for i, x := range list {
		// The mapped value stays a thunk: mapping does not force anything.
		result[i] = f.Apply(w, x)
	}
	return ListValue(result)
}

func bPartition(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	list := assertList(w, args[1].Eval(w))
	right := make(NixList, 0, len(list))
	wrong := make(NixList, 0, len(list))
	for _, x := range list {
		if assertBool(w, f.Apply(w, x).Eval(w)) {
			right = append(right, x)
		} else {
			wrong = append(wrong, x)
		}
	}
	return pair(w, symRight, ListValue(right), symWrong, ListValue(wrong))
}

// bSort sorts with a strict less-than comparator, keeping equal elements in
// their original order as Nix does.
func bSort(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	list := assertList(w, args[1].Eval(w))
	result := make(NixList, len(list))
	copy(result, list)
	sort.SliceStable(result, func(i, j int) bool {
		return bool(assertBool(w, apply2(w, f, result[i], result[j]).Eval(w)))
	})
	return ListValue(result)
}

func bTail(w *worker, args ...*Expression) NixValue {
	list := assertList(w, args[0].Eval(w))
	if len(list) == 0 {
		w.throwf(ErrEval, "cannot take the tail of an empty list")
	}
	return ListValue(list[1:])
}
