package eval

import (
	"testing"
)

// The store paths below were produced by Nix for the same expressions, so the
// test pins that the derivation hashing stays byte-for-byte compatible.

func TestDerivationPaths(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"simple drvPath",
			`(builtins.derivation { name = "foo"; builder = "/bin/sh"; system = "x86_64-linux"; }).drvPath`,
			`"/nix/store/9d794zbx10n43fa6zaqf33pfymdrm4r2-foo.drv"`,
		},
		{
			"simple outPath",
			`(builtins.derivation { name = "foo"; builder = "/bin/sh"; system = "x86_64-linux"; }).outPath`,
			`"/nix/store/xpcvxsx5sw4rbq666blz6sxqlmsqphmr-foo"`,
		},
		{
			"input derivation",
			`let a = builtins.derivation { name = "dep"; builder = "/bin/true"; system = "x86_64-linux"; };
			 in (builtins.derivation { name = "top"; builder = "/bin/sh"; system = "x86_64-linux"; dep = a.drvPath; }).drvPath`,
			`"/nix/store/w2v5c9ba4slr5jdvzhnqlh5sccf84xy4-top.drv"`,
		},
		{
			"input by outPath",
			`let a = builtins.derivation { name = "dep"; builder = "/bin/true"; system = "x86_64-linux"; };
			 in (builtins.derivation { name = "top"; builder = "/bin/sh"; system = "x86_64-linux"; dep = a.outPath; }).drvPath`,
			`"/nix/store/9za4y6zmcy6kdpg9958ir48xkar20w5a-top.drv"`,
		},
		{
			"multi-output out",
			`let a = builtins.derivation { name = "dep"; builder = "/bin/true"; system = "x86_64-linux"; };
			 in (builtins.derivation { name = "top"; builder = "/bin/sh"; system = "x86_64-linux"; outputs = [ "out" "dev" ]; dep = a.drvPath; }).outPath`,
			`"/nix/store/6pvma8zhy82ibikh4mjrjkmr3aszjfhs-top"`,
		},
		{
			"multi-output dev",
			`let a = builtins.derivation { name = "dep"; builder = "/bin/true"; system = "x86_64-linux"; };
			 in (builtins.derivation { name = "top"; builder = "/bin/sh"; system = "x86_64-linux"; outputs = [ "out" "dev" ]; dep = a.drvPath; }).dev.outPath`,
			`"/nix/store/7ra3lknchpdc1xlkxdsmd0n8pnm9yqid-top-dev"`,
		},
		{
			"fixed-output drvPath",
			`(builtins.derivation { name = "fod"; builder = "/bin/sh"; system = "x86_64-linux";
			   outputHash = "0000000000000000000000000000000000000000000000000000"; outputHashAlgo = "sha256"; outputHashMode = "flat"; }).drvPath`,
			`"/nix/store/c96xwi096xizcwkc2j8n5v7gicy73hr5-fod.drv"`,
		},
		{
			"fixed-output outPath",
			`(builtins.derivation { name = "fod"; builder = "/bin/sh"; system = "x86_64-linux";
			   outputHash = "0000000000000000000000000000000000000000000000000000"; outputHashAlgo = "sha256"; outputHashMode = "flat"; }).outPath`,
			`"/nix/store/59209gx1b93k17d049isa072bka4iv8w-fod"`,
		},
		{
			"structured-attrs drvPath",
			`(builtins.derivation { name = "sa"; builder = "/bin/sh"; system = "x86_64-linux";
			   __structuredAttrs = true; foo = "bar"; num = 3; list = [ 1 2 ]; nested = { x = 1; }; }).drvPath`,
			`"/nix/store/hr0lqcjinhk4jwyy85yilgaa44fw4z93-sa.drv"`,
		},
		{
			"structured-attrs outPath",
			`(builtins.derivation { name = "sa"; builder = "/bin/sh"; system = "x86_64-linux";
			   __structuredAttrs = true; foo = "bar"; num = 3; list = [ 1 2 ]; nested = { x = 1; }; }).outPath`,
			`"/nix/store/prp16lg0jg0rz4zfkhrshg4acnffw82z-sa"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := evalPrint(t, test.src)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestDerivationJSON(t *testing.T) {
	val, err := EvalString(`let a = builtins.derivation { name = "dep"; builder = "/bin/true"; system = "x86_64-linux"; };
		in builtins.derivation { name = "top"; builder = "/bin/sh"; system = "x86_64-linux"; outputs = [ "out" "dev" ]; args = [ "-c" "echo hello\nworld" ]; dep = a.drvPath; }`)
	if err != nil {
		t.Fatal(err)
	}
	d := DerivationOf(val)
	if d == nil {
		t.Fatal("value is not a derivation")
	}
	if want := "/nix/store/qzikiiliib9fs3y3l08j5wbxnf3pw1x4-top.drv"; d.DrvPath() != want {
		t.Errorf("drvPath = %s, want %s", d.DrvPath(), want)
	}
	got, err := d.JSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"args":["-c","echo hello\nworld"],"builder":"/bin/sh","env":{"builder":"/bin/sh","dep":"/nix/store/v9ak4484q7m2q36wrjh2rvpks78h1156-dep.drv","dev":"/nix/store/lmdhqcfyd6n20ghz42bkjb9zhwkndww8-top-dev","name":"top","out":"/nix/store/w2hnnycf7gbizkn40kbg7qpnx7fmsyv8-top","outputs":"out dev","system":"x86_64-linux"},"inputs":{"drvs":{"v9ak4484q7m2q36wrjh2rvpks78h1156-dep.drv":{"dynamicOutputs":{},"outputs":["out"]}},"srcs":["v9ak4484q7m2q36wrjh2rvpks78h1156-dep.drv"]},"name":"top","outputs":{"dev":{"path":"lmdhqcfyd6n20ghz42bkjb9zhwkndww8-top-dev"},"out":{"path":"w2hnnycf7gbizkn40kbg7qpnx7fmsyv8-top"}},"system":"x86_64-linux","version":4}`
	if string(got) != want {
		t.Errorf("JSON mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestDerivationStrict(t *testing.T) {
	got, err := evalPrint(t, `builtins.derivationStrict { name = "foo"; builder = "/bin/sh"; system = "x86_64-linux"; }`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{ drvPath = "/nix/store/9d794zbx10n43fa6zaqf33pfymdrm4r2-foo.drv"; out = "/nix/store/xpcvxsx5sw4rbq666blz6sxqlmsqphmr-foo"; }`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestDerivationBuiltins(t *testing.T) {
	for _, test := range [][2]string{
		{`builtins.placeholder "out"`, `"/1rz4g4znpzjwh1xymhjpm42vipw92pr73vdgl6xs1hycac8kf2n9"`},
		{`builtins.toFile "foo.txt" "hello"`, `"/nix/store/m0iyz1rzcgxw68h6skk8b9sg0p0napwc-foo.txt"`},
		{`builtins.hashString "sha256" "hello"`, `"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"`},
		{`builtins.baseNameOf "a/b/c/"`, `"c"`},
		{`builtins.dirOf "a/b/c"`, `"a/b"`},
		{`builtins.dirOf "a"`, `"."`},
		{`builtins.dirOf "/a"`, `"/"`},
		{`builtins.unsafeDiscardStringContext (builtins.derivation { name = "foo"; builder = "/bin/sh"; system = "x86_64-linux"; }).outPath`,
			`"/nix/store/xpcvxsx5sw4rbq666blz6sxqlmsqphmr-foo"`},
		{`builtins.hasContext (builtins.derivation { name = "foo"; builder = "/bin/sh"; system = "x86_64-linux"; }).outPath`, `true`},
		{`builtins.hasContext "plain"`, `false`},
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

// TestDerivationSelfReference pins that a derivation's self-referential
// structure — out refers back to the derivation — does not make strict
// printing or deepSeq recurse forever.
func TestDerivationSelfReference(t *testing.T) {
	src := `builtins.derivation { name = "foo"; builder = "/bin/sh"; system = "x86_64-linux"; }`
	if got, err := evalPrint(t, `builtins.deepSeq (`+src+`) 1`); err != nil || got != "1" {
		t.Errorf("deepSeq = %q, %v", got, err)
	}
	val, err := EvalString(src)
	if err != nil {
		t.Fatal(err)
	}
	if s, err := Print(val, -1); err != nil || s == "" {
		t.Errorf("strict print = %q, %v", s, err)
	}
}
