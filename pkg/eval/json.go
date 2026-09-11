package eval

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
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
		return jsonFloat(x.Float())
	case KindBool:
		return x.Bool()
	case KindString:
		s := x.Str()
		if ctx != nil && s.extra != nil {
			*ctx = append(*ctx, s.extra.Context...)
		}
		return s.Content
	case KindPath:
		return x.Path().String()
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
			s := t.coerceToString(w, false)
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

// jsonFloat renders a float the way Nix's JSON writer (nlohmann::json) does,
// which differs from Go's: an integral float keeps a ".0", and the switch to
// exponent notation happens at a different magnitude.
type jsonFloat float64

func (f jsonFloat) MarshalJSON() ([]byte, error) {
	return []byte(formatJSONFloat(float64(f))), nil
}

// formatJSONFloat is nlohmann's to_chars: the shortest round-trip digits from
// Go, laid out by nlohmann's format_buffer rules (fixed for a decimal point in
// (-4, 15], scientific otherwise).
func formatJSONFloat(f float64) string {
	sign := ""
	if math.Signbit(f) {
		sign, f = "-", -f
	}
	if f == 0 {
		return sign + "0.0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	mant, expStr, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.Replace(mant, ".", "", 1)
	// value = digits * 10^decExp, with the point after decExp more digits.
	decExp := exp - (len(digits) - 1)
	return sign + formatDecimal(digits, decExp)
}

// formatDecimal is nlohmann's format_buffer with min_exp -4 and max_exp 15.
func formatDecimal(digits string, decExp int) string {
	k := len(digits)
	n := k + decExp
	switch {
	case k <= n && n <= 15:
		// 123 -> 123000.0
		return digits + strings.Repeat("0", n-k) + ".0"
	case 0 < n && n <= 15:
		// 123456 -> 123.456
		return digits[:n] + "." + digits[n:]
	case -4 < n && n <= 0:
		// 0.00123
		return "0." + strings.Repeat("0", -n) + digits
	}
	mant := digits
	if k > 1 {
		mant = digits[:1] + "." + digits[1:]
	}
	return mant + "e" + formatExponent(n-1)
}

func formatExponent(e int) string {
	sign := "+"
	if e < 0 {
		sign, e = "-", -e
	}
	// nlohmann prints at least two exponent digits, for printf("%g") compat.
	s := strconv.Itoa(e)
	if len(s) < 2 {
		s = "0" + s
	}
	return sign + s
}

// marshalJSON renders a native value as JSON the way Nix does: compact, with
// keys sorted and without Go's HTML escaping (which would turn ">" into
// "\u003e").
func marshalJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
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
	out, err := marshalJSON(ValueToNative(w, args[0].Eval(w)))
	if err != nil {
		w.throwf(ErrEval, "cannot serialise to JSON: %s", err)
	}
	return String(out)
}
