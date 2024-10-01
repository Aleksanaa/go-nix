package main

import (
	"testing"

	"github.com/alecthomas/assert"
	"github.com/orivej/go-nix/nix/eval"
	"github.com/orivej/go-nix/nix/parser"
)

func TestEval(t *testing.T) {
	for _, test := range [][2]string{
		{`{ "abc" = 1; }`, `{ abc = ...; }`},
		{`{ abc = 1; }`, `{ abc = ...; }`},
		{`{ a = 1; }.a`, `1`},
		{`{ a."b".${"c"} = 1; }."${"a"}".${"b"}.c`, `1`},
		{`(rec { a = 2; b = a; }).b`, `2`},
		{`(rec { a.b.c = 2; b = a; }).b`, `{ b = { c = ...; }; }`},
		{`rec { a = rec { d = b; }; b = 3; }.a.d`, `3`},
		{`let a = 1; b = 2; in b`, `2`},
		{`let a = 1; in let b = 2; in a`, `1`},
		{`let a = 1; in { inherit a; }.a`, `1`},
		{`let a.c = 1; in let inherit (a) c; in c`, `1`},
		{`let a = 1; b = a; in b`, `1`},
		{`with { a = 1; }; a`, `1`},
		{`let a = 2; in with { a = 1; }; a`, `2`},
		{`with { a = 2; }; with { a = 1; }; a`, `1`},
	} {
		pr, err := parser.ParseString(test[0])
		assert.NoError(t, err)
		s := eval.ValueString(eval.ParseResult(pr))
		assert.Equal(t, test[1], s)
	}
}
