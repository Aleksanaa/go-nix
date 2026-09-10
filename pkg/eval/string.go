package eval

import (
	"strings"
	"sync/atomic"
)

// NixString is a string together with the build inputs it references.
//
// Almost no string references anything, and a string is one of the most
// numerous values there is, so what a string may carry besides its content
// lives behind a pointer that most of them never allocate.
type NixString struct {
	Content string
	// sym is the interned name of the content, or zero until something asks
	// for it. Strings are used as attribute names in pairs of operations —
	// attrNames hands them out, getAttr and `.${name}` look them up — and
	// interning the same content over again was the largest cost of an
	// attribute-heavy evaluation.
	// It is read and written atomically: a string is a value like any other,
	// so two workers can hold the same one and both want its name.
	sym   int32
	extra *stringExtra
}

// intern returns the symbol naming this string's content, interning it at
// most once per string.
//
// Two workers can hold the same string and both ask, so the cache is read and
// written atomically. Which of them wins does not matter: interning the same
// content twice gives the same symbol, so the loser stores what the winner
// stored.
func (str *NixString) intern() Sym {
	if sym := atomic.LoadInt32(&str.sym); sym != 0 {
		return Sym(sym)
	}
	sym := Intern(str.Content)
	atomic.StoreInt32(&str.sym, int32(sym))
	return sym
}

// stringSym makes a string that is already known to be the name sym, so that
// reading it back as a name costs nothing.
func stringSym(s string, sym Sym) *NixString {
	return &NixString{Content: s, sym: int32(sym)}
}

// stringExtra is what a string carries besides its content: the derivations it
// refers to, and the values interpolated from the evaluator itself — such as
// the Nix version — that make it non-reproducible.
type stringExtra struct {
	Context    []stringContext
	Impurities map[string]string
}

// newString makes a plain string.
func newString(s string) *NixString { return &NixString{Content: s} }

// withContext makes a string with src's content replaced but its context and
// impurities kept, as baseNameOf and dirOf do.
func withContext(content string, src *NixString) *NixString {
	s := newString(content)
	if src.extra == nil {
		return s
	}
	e := &stringExtra{}
	if len(src.extra.Context) != 0 {
		e.Context = append([]stringContext(nil), src.extra.Context...)
	}
	if len(src.extra.Impurities) != 0 {
		e.Impurities = make(map[string]string, len(src.extra.Impurities))
		for k, v := range src.extra.Impurities {
			e.Impurities[k] = v
		}
	}
	s.extra = e
	return s
}

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
