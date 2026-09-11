package eval

import (
	"encoding/json"
	"math"

	"github.com/aleksanaa/go-nix/pkg/nixjson"
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
	return valueToNative(w, x, nil)
}

// valueToNative converts a value the way Nix's printValueAsJSON does: a set
// with a __toString or outPath attribute serialises as its string form, which
// is how derivations end up as store paths, and every other set becomes an
// object. References the strings carry are appended to ctx, which the
// derivation builder needs to collect its inputs.
func valueToNative(w *worker, x NixValue, ctx *[]stringContext) any {
	switch x.Kind() {
	case KindNull:
		return nil
	case KindInt:
		return x.Int()
	case KindFloat:
		return nixjson.Float(x.Float())
	case KindBool:
		return x.Bool()
	case KindString:
		s := x.Str()
		if ctx != nil && s.extra != nil {
			*ctx = append(*ctx, s.extra.Context...)
		}
		return s.Content
	case KindPath:
		s := copyPathToStore(w, x.Path().String())
		if ctx != nil && s.extra != nil {
			*ctx = append(*ctx, s.extra.Context...)
		}
		return s.Content
	case KindList:
		list := x.List()
		result := make([]any, len(list))
		for i, el := range list {
			result[i] = valueToNative(w, el.Eval(w), ctx)
		}
		return result
	case KindSet:
		t := x.Set()
		if t.Has(symToString) || t.Has(symOutPath) {
			s := t.coerceToString(w, false, true)
			if ctx != nil && s.extra != nil {
				*ctx = append(*ctx, s.extra.Context...)
			}
			return s.Content
		}
		result := make(map[string]any, t.Len())
		for _, a := range t.attrs {
			result[a.sym.String()] = valueToNative(w, a.x.Eval(w), ctx)
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
	out, err := nixjson.Marshal(ValueToNative(w, args[0].Eval(w)))
	if err != nil {
		w.throwf(ErrEval, "cannot serialise to JSON: %s", err)
	}
	return String(string(out))
}
