package eval

import (
	"sort"
	"strings"
)

// The string-context builtins: reading and writing the references a string
// carries. getContext and appendContext round-trip the same representation, a
// set keyed by store path, and the output-dependency builtins narrow or widen
// a single reference.

// bGetContext implements builtins.getContext: the string's references as a set
// keyed by store path, each mapping to which of { path, allOutputs, outputs }
// the reference carries.
func bGetContext(w *worker, args ...*Expression) NixValue {
	str := assertString(w, args[0].Eval(w))

	type info struct {
		path       bool
		allOutputs bool
		outputs    []string
	}
	infos := map[string]*info{}
	get := func(key string) *info {
		i, ok := infos[key]
		if !ok {
			i = &info{}
			infos[key] = i
		}
		return i
	}
	if str.extra != nil {
		for _, c := range str.extra.Context {
			switch {
			case c.deep:
				get(c.drvPath).allOutputs = true
			case c.drvPath != "":
				get(c.drvPath).outputs = append(get(c.drvPath).outputs, c.out)
			case c.path != "":
				get(c.path).path = true
			}
		}
	}

	set := NewSet(len(infos))
	for key, i := range infos {
		sort.Strings(i.outputs)
		sub := NewSet(3)
		if i.path {
			sub.Bind1(symPath, value(w, True))
		}
		if i.allOutputs {
			sub.Bind1(symAllOutputs, value(w, True))
		}
		if len(i.outputs) != 0 {
			list := make(NixList, len(i.outputs))
			for j, o := range i.outputs {
				list[j] = value(w, String(o))
			}
			sub.Bind1(symOutputs, value(w, ListValue(list)))
		}
		set.Bind1(Intern(key), value(w, SetValue(sub.finish(w))))
	}
	return SetValue(set.finish(w))
}

// bAppendContext implements builtins.appendContext: the string with the given
// references added to whatever it already carried.
func bAppendContext(w *worker, args ...*Expression) NixValue {
	str := assertString(w, args[0].Eval(w))
	ctx := assertSet(w, args[1].Eval(w))

	result := newString(str.Content)
	if str.extra != nil {
		result.extra = &stringExtra{Context: append([]stringContext(nil), str.extra.Context...)}
	}

	for _, a := range ctx.attrs {
		key := a.sym.String()
		if !strings.HasPrefix(key, "/nix/store/") {
			w.throwf(ErrEval, "context key '%s' is not a store path", key)
		}
		val := assertSet(w, a.x.Eval(w))

		if x, ok := val.Get(symPath); ok && assertBool(w, x.Eval(w)) {
			result.append(stringContext{path: key})
		}
		if x, ok := val.Get(symAllOutputs); ok && assertBool(w, x.Eval(w)) {
			if !strings.HasSuffix(key, ".drv") {
				w.throwf(ErrEval, "tried to add all-outputs context of %s, which is not a derivation, to a string", key)
			}
			result.append(stringContext{drvPath: key, deep: true})
		}
		if x, ok := val.Get(symOutputs); ok {
			list := assertList(w, x.Eval(w))
			if len(list) != 0 && !strings.HasSuffix(key, ".drv") {
				w.throwf(ErrEval, "tried to add derivation output context of %s, which is not a derivation, to a string", key)
			}
			for _, el := range list {
				out := assertString(w, el.Eval(w)).Content
				result.append(stringContext{drvPath: key, out: out})
			}
		}
	}
	return StrValue(result)
}

// append adds one reference to a string's context.
func (s *NixString) append(c stringContext) {
	if s.extra == nil {
		s.extra = &stringExtra{}
	}
	s.extra.Context = append(s.extra.Context, c)
}

// bAddDrvOutputDependencies implements builtins.addDrvOutputDependencies: a
// string whose single opaque derivation-path reference becomes a whole-
// derivation (deep) reference.
func bAddDrvOutputDependencies(w *worker, args ...*Expression) NixValue {
	str := CoerceToString(w, args[0].Eval(w))
	var ctx []stringContext
	if str.extra != nil {
		ctx = str.extra.Context
	}
	if len(ctx) != 1 {
		w.throwf(ErrEval, "context of string '%s' must have exactly one element, but has %d", str.Content, len(ctx))
	}
	c := ctx[0]
	switch {
	case c.deep:
		// Idempotent: already a deep reference.
	case c.drvPath != "":
		w.throwf(ErrEval, "`addDrvOutputDependencies` can only act on derivations, not on a derivation output such as '%s'", c.out)
	case c.path != "":
		if !strings.HasSuffix(c.path, ".drv") {
			w.throwf(ErrEval, "path '%s' is not a derivation", c.path)
		}
		c = stringContext{drvPath: c.path, deep: true}
	}
	return StrValue(stringWithContext(str.Content, c))
}

// bUnsafeDiscardOutputDependency implements builtins.unsafeDiscardOutputDependency:
// a string whose deep references become plain derivation-path references.
func bUnsafeDiscardOutputDependency(w *worker, args ...*Expression) NixValue {
	str := CoerceToString(w, args[0].Eval(w))
	result := newString(str.Content)
	if str.extra != nil {
		for _, c := range str.extra.Context {
			if c.deep {
				c = stringContext{path: c.drvPath}
			}
			result.append(c)
		}
	}
	return StrValue(result)
}
