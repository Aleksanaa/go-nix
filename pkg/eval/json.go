package eval

import (
	"encoding/json"
	"math"
)

// ValueFromNative converts a value decoded from JSON into a Nix value.
func ValueFromNative(x any) NixValue {
	switch t := x.(type) {
	case nil:
		return Null
	case float64:
		// JSON has one number type; keep whole numbers as Nix integers.
		if math.Mod(t, 1.0) == 0 && math.Abs(t) < math.MaxInt64 {
			return NixInt(int64(t))
		}
		return NixFloat(t)
	case bool:
		return NixBool(t)
	case string:
		return String(t)
	case []any:
		result := make(NixList, len(t))
		for i, val := range t {
			result[i] = value(ValueFromNative(val))
		}
		return result
	case map[string]any:
		result := make(NixSet, len(t))
		for key, val := range t {
			result[Intern(key)] = value(ValueFromNative(val))
		}
		return result
	}
	throwf(ErrType, "cannot convert a Go value of type %T to a Nix value", x)
	return nil
}

// ValueToNative converts a Nix value into a value the JSON encoder accepts,
// forcing it completely.
func ValueToNative(x NixValue) any {
	switch t := x.(type) {
	case *NixNull:
		return nil
	case NixInt:
		return int64(t)
	case NixFloat:
		return float64(t)
	case NixBool:
		return bool(t)
	case *NixString:
		return t.Content
	case *NixPath:
		return t.String()
	case NixList:
		result := make([]any, len(t))
		for i, x := range t {
			result[i] = ValueToNative(x.Eval())
		}
		return result
	case NixSet:
		// A set with a __toString or outPath attribute serialises as its
		// string form, which is how derivations end up as store paths.
		if _, ok := t[symToString]; ok {
			return t.coerceToString(false).Content
		}
		if _, ok := t[symOutPath]; ok {
			return t.coerceToString(false).Content
		}
		result := make(map[string]any, len(t))
		for sym, x := range t {
			result[sym.String()] = ValueToNative(x.Eval())
		}
		return result
	}
	throwf(ErrType, "cannot convert %s to JSON", anTypeName(x))
	return nil
}

func bFromJSON(args ...*Expression) NixValue {
	str := assertString(args[0].Eval())
	var parsed any
	if err := json.Unmarshal([]byte(str.Content), &parsed); err != nil {
		throwf(ErrEval, "cannot parse JSON: %s", err)
	}
	return ValueFromNative(parsed)
}

func bToJSON(args ...*Expression) NixValue {
	b, err := json.Marshal(ValueToNative(args[0].Eval()))
	if err != nil {
		throwf(ErrEval, "cannot serialise to JSON: %s", err)
	}
	return String(string(b))
}
