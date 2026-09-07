package eval

import "strings"

// NixString is a string together with the build inputs it references.
type NixString struct {
	Content string
	Context []*Derivation
	// Impurities records values interpolated from the evaluator itself, such
	// as the Nix version, that make the string non-reproducible:
	// { "2.18": "reference to Nix version" }.
	Impurities map[string]string
}

// String makes a plain string value.
func String(s string) *NixString { return &NixString{Content: s} }

func (str *NixString) Print(recurse int) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`)
	return `"` + r.Replace(str.Content) + `"`
}

func (str *NixString) Compare(val NixValue) bool {
	str2, ok := val.(*NixString)
	return ok && str.Content == str2.Content
}

// absorb takes over the context and impurities of another string, which is
// what interpolating it into this one means.
func (str *NixString) absorb(other *NixString) {
	if len(other.Context) != 0 {
		str.Context = append(str.Context, other.Context...)
	}
	for name, reason := range other.Impurities {
		if str.Impurities == nil {
			str.Impurities = make(map[string]string, len(other.Impurities))
		}
		str.Impurities[name] = reason
	}
}

// Concat joins two strings, keeping the context of both.
func (str *NixString) Concat(other *NixString) *NixString {
	result := &NixString{Content: str.Content + other.Content}
	result.absorb(str)
	result.absorb(other)
	return result
}
