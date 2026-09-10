package eval

import (
	"encoding/json"
	"math"
)

// ValueFromNative converts a value decoded from JSON into a Nix value.
func ValueFromNative(w *worker, x any) NixValue {
	switch t := x.(type) {
	case nil:
		return Null
	case float64:
		// JSON has one number type; keep whole numbers as Nix integers.
		if math.Mod(t, 1.0) == 0 && math.Abs(t) < math.MaxInt64 {
			return Int(int64(t))
		}
		return Float(t)
	case bool:
		return Bool(t)
	case string:
		return String(t)
	case []any:
		result := make(NixList, len(t))
		for i, val := range t {
			result[i] = value(w, ValueFromNative(w, val))
		}
		return ListValue(result)
	case map[string]any:
		result := NewSet(len(t))
		for key, val := range t {
			result.Bind1(Intern(key), value(w, ValueFromNative(w, val)))
		}
		return SetValue(result.keepFirst())
	}
	w.throwf(ErrType, "cannot convert a Go value of type %T to a Nix value", x)
	return NixValue{}
}

// ValueToNative converts a Nix value into a value the JSON encoder accepts,
// forcing it completely.
func ValueToNative(w *worker, x NixValue) any {
	switch x.Kind() {
	case KindNull:
		return nil
	case KindInt:
		return x.Int()
	case KindFloat:
		return x.Float()
	case KindBool:
		return x.Bool()
	case KindString:
		return x.Str().Content
	case KindPath:
		return x.Path().String()
	case KindList:
		list := x.List()
		result := make([]any, len(list))
		for i, el := range list {
			result[i] = ValueToNative(w, el.Eval(w))
		}
		return result
	case KindSet:
		t := x.Set()
		// A set with a __toString or outPath attribute serialises as its
		// string form, which is how derivations end up as store paths.
		if t.Has(symToString) {
			return t.coerceToString(w, false).Content
		}
		if t.Has(symOutPath) {
			return t.coerceToString(w, false).Content
		}
		result := make(map[string]any, t.Len())
		for _, a := range t.attrs {
			result[a.sym.String()] = ValueToNative(w, a.x.Eval(w))
		}
		return result
	}
	w.throwf(ErrType, "cannot convert %s to JSON", anTypeName(x))
	return nil
}

func bFromJSON(w *worker, args ...*Expression) NixValue {
	str := assertString(w, args[0].Eval(w))
	var parsed any
	if err := json.Unmarshal([]byte(str.Content), &parsed); err != nil {
		w.throwf(ErrEval, "cannot parse JSON: %s", err)
	}
	return ValueFromNative(w, parsed)
}

func bToJSON(w *worker, args ...*Expression) NixValue {
	b, err := json.Marshal(ValueToNative(w, args[0].Eval(w)))
	if err != nil {
		w.throwf(ErrEval, "cannot serialise to JSON: %s", err)
	}
	return String(string(b))
}
