package eval

import "sort"

func bAll(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	for _, x := range assertList(args[1].Eval()) {
		if !assertBool(f.Apply(x).Eval()) {
			return False
		}
	}
	return True
}

func bAny(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	for _, x := range assertList(args[1].Eval()) {
		if assertBool(f.Apply(x).Eval()) {
			return True
		}
	}
	return False
}

func bConcatLists(args ...*Expression) NixValue {
	lists := assertList(args[0].Eval())
	result := make(NixList, 0, len(lists))
	for _, x := range lists {
		result = append(result, assertList(x.Eval())...)
	}
	return result
}

func bElem(args ...*Expression) NixValue {
	val := args[0].Eval()
	for _, x := range assertList(args[1].Eval()) {
		if val.Compare(x.Eval()) {
			return True
		}
	}
	return False
}

func bElemAt(args ...*Expression) NixValue {
	list := assertList(args[0].Eval())
	i := int64(assertInt(args[1].Eval()))
	if i < 0 || i >= int64(len(list)) {
		throwf(ErrEval, "list index %d is out of bounds, the list has %d elements", i, len(list))
	}
	return list[i].Eval()
}

func bFilter(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	list := assertList(args[1].Eval())
	result := make(NixList, 0, len(list))
	for _, x := range list {
		if assertBool(f.Apply(x).Eval()) {
			result = append(result, x)
		}
	}
	return result
}

// bFoldl is builtins.foldl', which forces the accumulator at every step.
func bFoldl(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	acc := args[1]
	for _, x := range assertList(args[2].Eval()) {
		step := assertLambda(f.Apply(acc).Eval())
		acc = value(step.Apply(x).Eval())
	}
	return acc.Eval()
}

func bGenList(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	n := int64(assertInt(args[1].Eval()))
	if n < 0 {
		throwf(ErrEval, "cannot create a list of %d elements", n)
	}
	result := make(NixList, n)
	for i := range result {
		result[i] = f.Apply(value(NixInt(i)))
	}
	return result
}

func bHead(args ...*Expression) NixValue {
	list := assertList(args[0].Eval())
	if len(list) == 0 {
		throwf(ErrEval, "list index 0 is out of bounds, the list is empty")
	}
	return list[0].Eval()
}

func bLength(args ...*Expression) NixValue {
	return NixInt(len(assertList(args[0].Eval())))
}

func bMap(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	list := assertList(args[1].Eval())
	result := make(NixList, len(list))
	for i, x := range list {
		// The mapped value stays a thunk: mapping does not force anything.
		result[i] = f.Apply(x)
	}
	return result
}

func bPartition(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	list := assertList(args[1].Eval())
	right := make(NixList, 0, len(list))
	wrong := make(NixList, 0, len(list))
	for _, x := range list {
		if assertBool(f.Apply(x).Eval()) {
			right = append(right, x)
		} else {
			wrong = append(wrong, x)
		}
	}
	return NixSet{symRight: value(right), symWrong: value(wrong)}
}

// bSort sorts with a strict less-than comparator, keeping equal elements in
// their original order as Nix does.
func bSort(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	list := assertList(args[1].Eval())
	result := make(NixList, len(list))
	copy(result, list)
	sort.SliceStable(result, func(i, j int) bool {
		less := assertLambda(f.Apply(result[i]).Eval())
		return bool(assertBool(less.Apply(result[j]).Eval()))
	})
	return result
}

func bTail(args ...*Expression) NixValue {
	list := assertList(args[0].Eval())
	if len(list) == 0 {
		throwf(ErrEval, "cannot take the tail of an empty list")
	}
	return list[1:]
}
