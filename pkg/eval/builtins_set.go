package eval

import "slices"

func bAttrNames(args ...*Expression) NixValue {
	set := assertSet(args[0].Eval())
	result := make(NixList, 0, set.Len())
	for _, sym := range set.Keys() {
		result = append(result, value(StrValue(stringSym(sym.String(), sym))))
	}
	return ListValue(result)
}

func bAttrValues(args ...*Expression) NixValue {
	set := assertSet(args[0].Eval())
	result := make(NixList, 0, set.Len())
	for _, sym := range set.Keys() {
		x, _ := set.Get(sym)
		result = append(result, x)
	}
	return ListValue(result)
}

// bCatAttrs collects an attribute from every set of a list that has it.
func bCatAttrs(args ...*Expression) NixValue {
	sym := assertString(args[0].Eval()).intern()
	list := assertList(args[1].Eval())
	result := make(NixList, 0, len(list))
	for _, x := range list {
		if y, ok := assertSet(x.Eval()).Get(sym); ok {
			result = append(result, y)
		}
	}
	return ListValue(result)
}

func bFunctionArgs(args ...*Expression) NixValue {
	val := args[0].Eval()
	if val.Kind() != KindLambda {
		if val.IsLambda() {
			// A builtin has no formal arguments to report.
			return SetValue(NewSet(0))
		}
		throwf(ErrType, "value is %s while a function was expected", anTypeName(val))
	}
	f := val.Lambda().(*NixExprLambda)
	result := NewSet(len(f.Formal))
	for sym, def := range f.Formal {
		result.Bind1(sym, value(Bool(def != nil)))
	}
	return SetValue(result.finish())
}

func bGetAttr(args ...*Expression) NixValue {
	sym := assertString(args[0].Eval()).intern()
	set := assertSet(args[1].Eval())
	x, ok := set.Get(sym)
	if !ok {
		throwf(ErrMissingAttribute, "attribute '%s' missing", sym)
	}
	return x.Eval()
}

func bGroupBy(args ...*Expression) NixValue {
	f := assertLambda(args[0].Eval())
	list := assertList(args[1].Eval())
	groups := make(map[Sym]NixList)
	for _, x := range list {
		sym := assertString(f.Apply(x).Eval()).intern()
		groups[sym] = append(groups[sym], x)
	}
	result := NewSet(len(groups))
	for sym, group := range groups {
		result.Bind1(sym, value(ListValue(group)))
	}
	return SetValue(result.finish())
}

func bHasAttr(args ...*Expression) NixValue {
	sym := assertString(args[0].Eval()).intern()
	return Bool(assertSet(args[1].Eval()).Has(sym))
}

func bIntersectAttrs(args ...*Expression) NixValue {
	left := assertSet(args[0].Eval())
	right := assertSet(args[1].Eval())
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
func bListToAttrs(args ...*Expression) NixValue {
	list := assertList(args[0].Eval())
	nameSym, valueSym := Intern("name"), symValue
	result := NewSet(len(list))
	for _, x := range list {
		entry := assertSet(x.Eval())
		nameExpr, ok := entry.Get(nameSym)
		if !ok {
			throwf(ErrMissingAttribute, "attribute 'name' missing in a list element of listToAttrs")
		}
		valExpr, ok := entry.Get(valueSym)
		if !ok {
			throwf(ErrMissingAttribute, "attribute 'value' missing in a list element of listToAttrs")
		}
		result.Bind1(assertString(nameExpr.Eval()).intern(), valExpr)
	}
	return SetValue(result.keepFirst())
}

func bRemoveAttrs(args ...*Expression) NixValue {
	set := assertSet(args[0].Eval())
	names := assertList(args[1].Eval())
	drop := make([]Sym, 0, len(names))
	for _, x := range names {
		drop = append(drop, assertString(x.Eval()).intern())
	}
	result := make([]attr, 0, set.Len())
	for _, a := range set.attrs {
		if !slices.Contains(drop, a.sym) {
			result = append(result, a)
		}
	}
	return SetValue(setOf(result))
}
