package eval

func bAttrNames(args ...*Expression) NixValue {
	set := assertSet(args[0].Eval())
	result := make(NixList, 0, len(set))
	for _, sym := range set.Keys() {
		result = append(result, value(String(sym.String())))
	}
	return result
}

func bAttrValues(args ...*Expression) NixValue {
	set := assertSet(args[0].Eval())
	result := make(NixList, 0, len(set))
	for _, sym := range set.Keys() {
		result = append(result, set[sym])
	}
	return result
}

// bCatAttrs collects an attribute from every set of a list that has it.
func bCatAttrs(args ...*Expression) NixValue {
	sym := Intern(assertString(args[0].Eval()).Content)
	list := assertList(args[1].Eval())
	result := make(NixList, 0, len(list))
	for _, x := range list {
		if y, ok := assertSet(x.Eval())[sym]; ok {
			result = append(result, y)
		}
	}
	return result
}

func bFunctionArgs(args ...*Expression) NixValue {
	val := args[0].Eval()
	f, ok := val.(*NixExprLambda)
	if !ok {
		if _, ok := val.(NixLambda); ok {
			// A builtin has no formal arguments to report.
			return NixSet{}
		}
		throwf(ErrType, "value is %s while a function was expected", anTypeName(val))
	}
	result := make(NixSet, len(f.Formal))
	for sym, def := range f.Formal {
		result[sym] = value(NixBool(def != nil))
	}
	return result
}

func bGetAttr(args ...*Expression) NixValue {
	sym := Intern(assertString(args[0].Eval()).Content)
	set := assertSet(args[1].Eval())
	x, ok := set[sym]
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
		sym := Intern(assertString(f.Apply(x).Eval()).Content)
		groups[sym] = append(groups[sym], x)
	}
	result := make(NixSet, len(groups))
	for sym, group := range groups {
		result[sym] = value(group)
	}
	return result
}

func bHasAttr(args ...*Expression) NixValue {
	sym := Intern(assertString(args[0].Eval()).Content)
	_, ok := assertSet(args[1].Eval())[sym]
	return NixBool(ok)
}

func bIntersectAttrs(args ...*Expression) NixValue {
	left := assertSet(args[0].Eval())
	right := assertSet(args[1].Eval())
	result := make(NixSet, min(len(left), len(right)))
	// Iterate the smaller set, since only shared names can contribute.
	if len(right) <= len(left) {
		for sym, x := range right {
			if _, ok := left[sym]; ok {
				result[sym] = x
			}
		}
		return result
	}
	for sym := range left {
		if x, ok := right[sym]; ok {
			result[sym] = x
		}
	}
	return result
}

// bListToAttrs builds a set from a list of { name, value } sets. As in Nix,
// the first occurrence of a name wins.
func bListToAttrs(args ...*Expression) NixValue {
	list := assertList(args[0].Eval())
	nameSym, valueSym := Intern("name"), symValue
	result := make(NixSet, len(list))
	for _, x := range list {
		entry := assertSet(x.Eval())
		nameExpr, ok := entry[nameSym]
		if !ok {
			throwf(ErrMissingAttribute, "attribute 'name' missing in a list element of listToAttrs")
		}
		valExpr, ok := entry[valueSym]
		if !ok {
			throwf(ErrMissingAttribute, "attribute 'value' missing in a list element of listToAttrs")
		}
		if sym := Intern(assertString(nameExpr.Eval()).Content); result[sym] == nil {
			result[sym] = valExpr
		}
	}
	return result
}

func bRemoveAttrs(args ...*Expression) NixValue {
	set := assertSet(args[0].Eval())
	names := assertList(args[1].Eval())
	result := make(NixSet, len(set))
	for sym, x := range set {
		result[sym] = x
	}
	for _, x := range names {
		delete(result, Intern(assertString(x.Eval()).Content))
	}
	return result
}
