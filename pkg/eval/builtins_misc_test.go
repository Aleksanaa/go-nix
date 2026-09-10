package eval

import (
	"testing"
)

func TestMiscBuiltins(t *testing.T) {
	for _, test := range [][2]string{
		{`builtins.mapAttrs (n: v: v * 10) { a = 1; b = 2; }`, `{ a = 10; b = 20; }`},
		{`builtins.concatMap (x: [ x x ]) [ 1 2 ]`, `[ 1 1 2 2 ]`},
		{`builtins.zipAttrsWith (n: v: { ${n} = v; }) [ { a = "x"; } { a = "y"; b = "z"; } ]`,
			`{ a = { a = [ "x" "y" ]; }; b = { b = [ "z" ]; }; }`},
		{`builtins.parseDrvName "nix-0.12pre12876"`, `{ name = "nix"; version = "0.12pre12876"; }`},
		{`builtins.splitVersion "1.2.3pre"`, `[ "1" "2" "3" "pre" ]`},
		{`builtins.storeDir`, `"/nix/store"`},
		{`builtins.hashString "sha256" "hello"`, `"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"`},
		{`builtins.convertHash { hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="; toHashFormat = "nix32"; }`,
			`"0000000000000000000000000000000000000000000000000000"`},
		{`builtins.convertHash { hash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"; toHashFormat = "sri"; }`,
			`"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="`},
		// A set with a __functor attribute is callable.
		{`let f = { __functor = self: x: x * 2; }; in f 21`, `42`},
		// A null attribute name skips the binding.
		{`{ ${null} = 1; ${"y"} = 2; }`, `{ y = 2; }`},
	} {
		got, err := evalPrint(t, test[0])
		if err != nil {
			t.Errorf("%s: %v", test[0], err)
			continue
		}
		if got != test[1] {
			t.Errorf("%s: got %s, want %s", test[0], got, test[1])
		}
	}
}

func TestGenericClosure(t *testing.T) {
	got, err := evalPrint(t, `builtins.genericClosure {
	  startSet = [ { key = 5; } ];
	  operator = item: [ { key = if (item.key / 2) * 2 == item.key then item.key / 2 else 3 * item.key + 1; } ];
	}`)
	if err != nil {
		t.Fatal(err)
	}
	want := `[ { key = 5; } { key = 16; } { key = 8; } { key = 4; } { key = 2; } { key = 1; } ]`
	if got != want {
		t.Errorf("genericClosure = %s, want %s", got, want)
	}
}

func TestStringContextBuiltins(t *testing.T) {
	// A derivation's outPath carries a built-output reference, which
	// getContext reports and appendContext can reproduce.
	src := `builtins.derivation { name = "ctx"; builder = "/bin/sh"; system = "x86_64-linux"; }`
	got, err := evalPrint(t, `let d = `+src+`; in builtins.getContext d.outPath`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{ /nix/store/n96mls8jja99bb70ghnlxk8mdb5b51i9-ctx.drv = { outputs = [ "out" ]; }; }`
	if got != want {
		t.Errorf("getContext = %s, want %s", got, want)
	}

	got, err = evalPrint(t, `let d = `+src+`; in builtins.appendContext "plain" (builtins.getContext d.outPath)`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `"plain"` {
		t.Errorf("appendContext = %s, want %s", got, want)
	}
}

func TestLazyWithFixpoint(t *testing.T) {
	// A fixpoint whose body is `with self; …` must not force self when a name
	// resolves lexically.
	got, err := evalPrint(t, `let f = self: with self; { a = 1; b = 2; }; self = f self; in self.b`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2" {
		t.Errorf("got %s, want 2", got)
	}
}
