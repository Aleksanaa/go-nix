package eval

import "strings"

// NixString is a string together with the build inputs it references.
//
// Almost no string references anything, and a string is one of the most
// numerous values there is, so what a string may carry besides its content
// lives behind a pointer that most of them never allocate.
type NixString struct {
	Content string
	extra   *stringExtra
}

// stringExtra is what a string carries besides its content: the derivations it
// refers to, and the values interpolated from the evaluator itself — such as
// the Nix version — that make it non-reproducible.
type stringExtra struct {
	Context    []*Derivation
	Impurities map[string]string
}

// newString makes a plain string.
func newString(s string) *NixString { return &NixString{Content: s} }

func (str *NixString) Print() string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`)
	return `"` + r.Replace(str.Content) + `"`
}

// absorb takes over the context and impurities of another string, which is
// what interpolating it into this one means.
func (str *NixString) absorb(other *NixString) {
	if other.extra == nil {
		return
	}
	if str.extra == nil {
		str.extra = &stringExtra{}
	}
	if len(other.extra.Context) != 0 {
		str.extra.Context = append(str.extra.Context, other.extra.Context...)
	}
	for name, reason := range other.extra.Impurities {
		if str.extra.Impurities == nil {
			str.extra.Impurities = make(map[string]string, len(other.extra.Impurities))
		}
		str.extra.Impurities[name] = reason
	}
}

// Concat joins two strings, keeping the context of both.
func (str *NixString) Concat(other *NixString) *NixString {
	result := &NixString{Content: str.Content + other.Content}
	result.absorb(str)
	result.absorb(other)
	return result
}
