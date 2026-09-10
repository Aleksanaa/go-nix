package eval

import "slices"

func bAttrNames(w *worker, args ...*Expression) NixValue {
	set := assertSet(w, args[0].Eval(w))
	result := make(NixList, 0, set.Len())
	for _, sym := range set.Keys() {
		result = append(result, value(w, StrValue(stringSym(sym.String(), sym))))
	}
	return ListValue(result)
}

func bAttrValues(w *worker, args ...*Expression) NixValue {
	set := assertSet(w, args[0].Eval(w))
	result := make(NixList, 0, set.Len())
	for _, sym := range set.Keys() {
		x, _ := set.Get(sym)
		result = append(result, x)
	}
	return ListValue(result)
}

// bCatAttrs collects an attribute from every set of a list that has it.
func bCatAttrs(w *worker, args ...*Expression) NixValue {
	sym := assertString(w, args[0].Eval(w)).intern()
	list := assertList(w, args[1].Eval(w))
	result := make(NixList, 0, len(list))
	for _, x := range list {
		if y, ok := assertSet(w, x.Eval(w)).Get(sym); ok {
			result = append(result, y)
		}
	}
	return ListValue(result)
}

func bFunctionArgs(w *worker, args ...*Expression) NixValue {
	val := args[0].Eval(w)
	if val.Kind() != KindLambda {
		if val.IsLambda() {
			// A builtin has no formal arguments to report.
			return SetValue(NewSet(0))
		}
		w.throwf(ErrType, "value is %s while a function was expected", anTypeName(val))
	}
	f := val.Lambda().(*NixExprLambda)
	result := NewSet(len(f.Formal))
	for sym, def := range f.Formal {
		result.Bind1(sym, value(w, Bool(def != nil)))
	}
	return SetValue(result.finish(w))
}

func bGetAttr(w *worker, args ...*Expression) NixValue {
	sym := assertString(w, args[0].Eval(w)).intern()
	set := assertSet(w, args[1].Eval(w))
	x, ok := set.Get(sym)
	if !ok {
		w.throwf(ErrMissingAttribute, "attribute '%s' missing", sym)
	}
	return x.Eval(w)
}

func bGroupBy(w *worker, args ...*Expression) NixValue {
	f := assertLambda(w, args[0].Eval(w))
	list := assertList(w, args[1].Eval(w))
	groups := make(map[Sym]NixList)
	for _, x := range list {
		sym := assertString(w, f.Apply(w, x).Eval(w)).intern()
		groups[sym] = append(groups[sym], x)
	}
	result := NewSet(len(groups))
	for sym, group := range groups {
		result.Bind1(sym, value(w, ListValue(group)))
	}
	return SetValue(result.finish(w))
}

func bHasAttr(w *worker, args ...*Expression) NixValue {
	sym := assertString(w, args[0].Eval(w)).intern()
	return Bool(assertSet(w, args[1].Eval(w)).Has(sym))
}

func bIntersectAttrs(w *worker, args ...*Expression) NixValue {
	left := assertSet(w, args[0].Eval(w))
	right := assertSet(w, args[1].Eval(w))
	// Both are in order, so this walks them side by side and takes the value
	// from the right, as Nix does.
	result := make([]attr, 0, min(left.Len(), right.Len()))
	i, j := 0, 0
	for i < len(left.attrs) && j < len(right.attrs) {
		switch a, b := left.attrs[i], right.attrs[j]; {
		case a.sym < b.sym:
			i++
		case a.sym > b.sym:
			j++
		default:
			result, i, j = append(result, b), i+1, j+1
		}
	}
	return SetValue(setOf(result))
}

// bListToAttrs builds a set from a list of { name, value } sets. As in Nix,
// the first occurrence of a name wins.
func bListToAttrs(w *worker, args ...*Expression) NixValue {
	list := assertList(w, args[0].Eval(w))
	// Every element is forced below, to read a name out of it.
	wait := forceAll(w, list)
	result := NewSet(len(list))
	for _, x := range list {
		entry := assertSet(w, x.Eval(w))
		nameExpr, ok := entry.Get(symName)
		if !ok {
			w.throwf(ErrMissingAttribute, "attribute 'name' missing in a list element of listToAttrs")
		}
		valExpr, ok := entry.Get(symValue)
		if !ok {
			w.throwf(ErrMissingAttribute, "attribute 'value' missing in a list element of listToAttrs")
		}
		result.Bind1(assertString(w, nameExpr.Eval(w)).intern(), valExpr)
	}
	wait()
	return SetValue(result.keepFirst())
}

func bRemoveAttrs(w *worker, args ...*Expression) NixValue {
	set := assertSet(w, args[0].Eval(w))
	names := assertList(w, args[1].Eval(w))
	drop := make([]Sym, 0, len(names))
	for _, x := range names {
		drop = append(drop, assertString(w, x.Eval(w)).intern())
	}
	result := make([]attr, 0, set.Len())
	for _, a := range set.attrs {
		if !slices.Contains(drop, a.sym) {
			result = append(result, a)
		}
	}
	return SetValue(setOf(result))
}
