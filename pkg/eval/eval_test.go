package eval

import (
	"strings"
	"testing"
)

// testDepth prints deeply enough for the fixtures below to be readable while
// still terminating on a self-referential value.
const testDepth = 8

// evalPrint evaluates src and prints the result.
func evalPrint(t *testing.T, src string) (string, error) {
	t.Helper()
	val, err := EvalString(src)
	if err != nil {
		return "", err
	}
	return val.Print(mainWorker, testDepth), nil
}

func TestEval(t *testing.T) {
	for _, test := range [][2]string{
		// Literals and strings.
		{`.2`, `0.2`},
		{`1.`, `1`},
		{`3.232111`, `3.23211`},
		{`3.232116`, `3.23212`},
		{`323119232739.`, `3.23119e+11`},
		{`"a${"b"}c"`, `"abc"`},
		{`"a\nb"`, `"a\nb"`},

		// Sets, selection and recursion.
		{`{ "abc" = 1; }`, `{ abc = 1; }`},
		{`{ a = 1; }.a`, `1`},
		{`{ a."b".${"c"} = 1; }."${"a"}".${"b"}.c`, `1`},
		{`(rec { a = 2; b = a; }).b`, `2`},
		{`(rec { a.b.c = 2; b = a; }).b`, `{ b = { c = 2; }; }`},
		{`rec { a = rec { d = b; }; b = 3; }.a.d`, `3`},
		{`{ a = 1; }.b or 2`, `2`},
		{`{ a = 1; }.a.b or 2`, `2`},
		{`{ a.b = 1; } ? a.b`, `true`},
		{`{ a.b = 1; } ? a.c`, `false`},
		{`{ a = 1; } ? "a"`, `true`},
		{`{ a = throw "boom"; } ? a`, `true`},
		{`{ a.b = throw "boom"; } ? a.b`, `true`},
		{`1 ? a`, `false`},
		{`{ a = 1; } // { a = 2; b = 3; }`, `{ a = 2; b = 3; }`},

		// let, with and inherit.
		{`let a = 1; b = 2; in b`, `2`},
		{`let a = 1; in let b = 2; in a`, `1`},
		{`let a = 1; in { inherit a; }.a`, `1`},
		{`let a.c = 1; in let inherit (a) c; in c`, `1`},
		{`let a = 1; b = a; in b`, `1`},
		{`with { a = 1; }; a`, `1`},
		{`let a = 2; in with { a = 1; }; a`, `2`},
		{`with { a = 2; }; with { a = 1; }; a`, `1`},
		{`with { a = 1; }; with { b = 2; }; a`, `1`},
		// A with-set is only forced when a name is looked up in it.
		{`with (throw "boom"); 1`, `1`},
		{`with { x = throw "boom"; y = 1; }; y`, `1`},

		// Functions.
		{`(a: a) 1`, `1`},
		{`({a,b,c}: b) { a = 1; b = 2; c = 3; }`, `2`},
		{`({a,b,...}: b) { a = 1; b = 2; c = 3; }`, `2`},
		{`({a,b,d?4,...}: d) { a = 1; b = 2; c = 3; }`, `4`},
		{`(({a,b?3,...}@arg: arg) { a = 1; b = 2; c = 3; }).b`, `2`},
		{`(({a,b?arg}@arg: b) { a = 2; }).a`, `2`},
		{`(a: b: a: b) 1 2 3`, `2`},
		{`(a: { a ? 2 }: a) 1 {}`, `2`},

		// Operators.
		{`1 + 2`, `3`},
		{`1 + 2.5`, `3.5`},
		{`"a" + "b"`, `"ab"`},
		// `+` coerces a set through outPath or __toString, so a derivation
		// can be joined with a subdirectory. The first operand decides the
		// result: a string stays a string, a path stays a path.
		{`{ outPath = "/a"; } + "/b"`, `"/a/b"`},
		{`"/a" + { outPath = "/b"; }`, `"/a/b"`},
		{`{ __toString = self: "a"; } + "b"`, `"ab"`},
		{`{ outPath = { outPath = "/a"; }; } + "b"`, `"/ab"`},
		// A path concatenates with no separator, so `./x + "y"` names `./xy`.
		{`builtins.baseNameOf (./x + "y")`, `"xy"`},
		{`builtins.baseNameOf (./a + "/b")`, `"b"`},
		{`7 - 2 * 3`, `1`},
		{`6 / 3`, `2`},
		{`- 2 + 1`, `-1`},
		{`!false`, `true`},
		{`1 < 2`, `true`},
		{`"a" < "b"`, `true`},
		{`[ 1 2 ] < [ 1 3 ]`, `true`},
		{`[ 1 ] < [ 1 2 ]`, `true`},
		{`true && false`, `false`},
		{`false || true`, `true`},
		{`false -> false`, `true`},
		{`{a.b = 1;} == { a = { b = 1; }; }`, `true`},
		{`2.47207e+17 == 247207427047107403`, `false`},
		{`1 == 1.0`, `true`},
		{`[[1][2 2]] ++ [[3 3 3]] == [[1][2 2][3 3 3]]`, `true`},
		// A set compares equal to itself even when it holds a function, as
		// Nix's eqValues reports before comparing structurally.
		{`let s = { a.b = (a: a); }; in s == s`, `true`},
		// Two distinct sets with the same structure do not, because the
		// function inside them is incomparable.
		{`let s = { a = (a: a); }; t = { a = (a: a); }; in s == t`, `false`},
		{`let f = a: a; in f == f`, `false`},
		{`{ a = 1; } == { b = 1; }`, `false`},

		// Laziness: the unused parts must never be evaluated.
		{`(a: 1) (throw "boom")`, `1`},
		{`{ a = throw "boom"; b = 2; }.b`, `2`},
		{`(builtins.head [ 1 (throw "boom") ])`, `1`},
		{`false && (throw "boom")`, `false`},
		{`true || (throw "boom")`, `true`},
		{`if true then 1 else throw "boom"`, `1`},
		{`builtins.length [ (throw "boom") ]`, `1`},

		// Control flow.
		{`if 1 < 2 then "y" else "n"`, `"y"`},
		{`assert 1 < 2; "ok"`, `"ok"`},

		// Builtins.
		{`builtins.add 1 2`, `3`},
		{`builtins.typeOf { }`, `"set"`},
		{`builtins.typeOf (a: a)`, `"lambda"`},
		{`builtins.attrNames { b = 1; a = 2; }`, `[ "a" "b" ]`},
		{`builtins.attrValues { b = 1; a = 2; }`, `[ 2 1 ]`},
		{`builtins.hasAttr "a" { a = 1; }`, `true`},
		{`builtins.getAttr "a" { a = 1; }`, `1`},
		{`builtins.removeAttrs { a = 1; b = 2; } [ "a" ]`, `{ b = 2; }`},
		{`builtins.intersectAttrs { a = 1; } { a = 2; b = 3; }`, `{ a = 2; }`},
		{`builtins.listToAttrs [ { name = "a"; value = 1; } ]`, `{ a = 1; }`},
		{`builtins.catAttrs "a" [ { a = 1; } { b = 2; } { a = 3; } ]`, `[ 1 3 ]`},
		{`map (x: x * 2) [ 1 2 3 ]`, `[ 2 4 6 ]`},
		{`builtins.filter (x: x > 1) [ 1 2 3 ]`, `[ 2 3 ]`},
		{`builtins.elem 2 [ 1 2 ]`, `true`},
		{`builtins.elem "a" [ "a" ]`, `true`},
		{`builtins.elemAt [ 1 2 3 ] 1`, `2`},
		{`builtins.tail [ 1 2 3 ]`, `[ 2 3 ]`},
		{`builtins.genList (x: x * x) 4`, `[ 0 1 4 9 ]`},
		{`builtins.foldl' (a: b: a + b) 0 [ 1 2 3 ]`, `6`},
		{`builtins.sort (a: b: a < b) [ 3 1 2 ]`, `[ 1 2 3 ]`},
		{`builtins.partition (x: x > 1) [ 1 2 ]`, `{ right = [ 2 ]; wrong = [ 1 ]; }`},
		// mapAttrs and zipAttrsWith defer applying f: forcing only the set
		// structure must not run a function whose value is a throw.
		{`builtins.attrNames (builtins.mapAttrs (n: throw "boom") { a = 1; })`, `[ "a" ]`},
		{`builtins.attrNames (builtins.zipAttrsWith (n: vs: throw "boom") [ { a = 1; } ])`, `[ "a" ]`},
		{`builtins.groupBy (x: x) [ "a" "b" "a" ]`, `{ a = [ "a" "a" ]; b = [ "b" ]; }`},
		{`builtins.concatLists [ [ 1 ] [ 2 ] ]`, `[ 1 2 ]`},
		{`builtins.concatStringsSep "," [ "a" "b" ]`, `"a,b"`},
		{`builtins.substring 1 2 "abcd"`, `"bc"`},
		{`builtins.substring 1 (0 - 1) "abcd"`, `"bcd"`},
		{`builtins.substring 9 1 "abcd"`, `""`},
		{`builtins.stringLength "abc"`, `3`},
		{`builtins.replaceStrings [ "a" ] [ "x" ] "banana"`, `"bxnxnx"`},
		{`builtins.match "a(b*)c" "abbc"`, `[ "bb" ]`},
		{`builtins.match "ab" "abc"`, `null`},
		{`builtins.split "," "a,b"`, `[ "a" [ ] "b" ]`},
		{`builtins.toString 1`, `"1"`},
		{`builtins.toString true`, `"1"`},
		{`builtins.toString null`, `""`},
		{`builtins.toString [ "a" "b" ]`, `"a b"`},
		{`builtins.toString { __toString = self: "x"; }`, `"x"`},
		{`"${{ outPath = "/nix/store/x"; }}"`, `"/nix/store/x"`},
		{`builtins.toJSON { a = [ 1 "b" ]; }`, `"{\"a\":[1,\"b\"]}"`},
		{`builtins.fromJSON "{\"a\":[1,null]}"`, `{ a = [ 1 null ]; }`},
		{`builtins.functionArgs ({ a, b ? 1 }: a)`, `{ a = false; b = true; }`},
		{`builtins.ceil 1.2`, `2`},
		{`builtins.floor 1.8`, `1`},
		{`builtins.ceil 2`, `2`},
		{`builtins.bitAnd 12 10`, `8`},
		{`builtins.lessThan "a" "b"`, `true`},
		{`builtins.compareVersions "1.0" "1.1"`, `-1`},
		{`builtins.compareVersions "2.3" "2.3"`, `0`},
		{`builtins.compareVersions "1.0" "1.0pre1"`, `1`},
		{`builtins.seq 1 2`, `2`},
		{`builtins.deepSeq [ 1 ] 2`, `2`},
		{`builtins.isNull null`, `true`},
		{`builtins.tryEval (throw "boom")`, `{ success = false; value = false; }`},
		{`builtins.tryEval 1`, `{ success = true; value = 1; }`},
		{`(builtins.tryEval (assert false; 1)).success`, `false`},
		{`builtins.builtins.isInt 1`, `true`},
		{`__isInt 1`, `true`},
	} {
		got, err := evalPrint(t, test[0])
		if err != nil {
			t.Errorf("%s\nunexpected error: %v", test[0], err)
			continue
		}
		if got != test[1] {
			t.Errorf("%s\n got: %s\nwant: %s", test[0], got, test[1])
		}
	}
}

// TestEvalErrors checks that failures are reported with the message Nix uses,
// rather than crashing the evaluator.
func TestEvalErrors(t *testing.T) {
	for _, test := range [][2]string{
		{`b`, `undefined variable 'b'`},
		{`{ a = 1; }.b`, `attribute 'b' missing`},
		{`builtins.getAttr "b" { }`, `attribute 'b' missing`},
		{`1 + "a"`, `value is a string while a number was expected`},
		{`"a" + 1`, `cannot coerce an integer to a string`},
		{`{ a = 1; } + "x"`, `cannot coerce a set to a string`},
		{`./x + (builtins.toFile "f" "c")`, `a string that refers to a store path cannot be appended to a path`},
		{`1 1`, `value is an integer while a function was expected`},
		{`(1).a`, `value is an integer while a set was expected`},
		{`if 1 then 2 else 3`, `value is an integer while a Boolean was expected`},
		{`1 / 0`, `division by zero`},
		{`{ a = 1; a = 2; }`, `attribute 'a' already defined`},
		{`{ a = 1; a.b = 2; }`, `attribute 'a' already defined`},
		// A set and a path that share a prefix merge, but a repeated leaf does
		// not, and neither does a set with a non-set.
		{`{ a.b = 1; a.b = 2; }`, `attribute 'a.b' already defined`},
		{`{ a = { b = 1; }; a.b = 2; }`, `attribute 'a.b' already defined`},
		{`{ a = { b = 1; }; a = { b = 2; }; }`, `attribute 'a.b' already defined`},
		{`let a = a; in a`, `infinite recursion encountered`},
		{`let a = b; b = a; in a`, `infinite recursion encountered`},
		{`rec { a = b; b = a; }.a`, `infinite recursion encountered`},
		{`throw "boom"`, `boom`},
		{`abort "stop"`, `evaluation aborted with the following error message: 'stop'`},
		{`assert 1 == 2; 1`, `assertion '1 == 2' failed`},
		{`({ a }: a) { }`, `function called without required argument 'a'`},
		{`({ a }: a) { a = 1; b = 2; }`, `function called with unexpected argument 'b'`},
		{`builtins.head [ ]`, `list index 0 is out of bounds, the list is empty`},
		{`builtins.elemAt [ 1 ] 5`, `list index 5 is out of bounds, the list has 1 elements`},
		{`"${1}"`, `cannot coerce an integer to a string`},
		{`builtins.toString (a: a)`, `cannot coerce a function to a string`},
		// tryEval must not swallow a type error.
		{`builtins.tryEval (1 + "a")`, `value is a string while a number was expected`},
	} {
		_, err := evalPrint(t, test[0])
		if err == nil {
			t.Errorf("%s\nexpected error %q, got none", test[0], test[1])
			continue
		}
		if !strings.Contains(err.Error(), test[1]) {
			t.Errorf("%s\n got: %v\nwant it to contain: %s", test[0], err, test[1])
		}
	}
}

// TestAttrPathMerge covers how the parser merges bindings that define the same
// attribute through a path and through a set literal. Nix merges the sets, so
// `{ a = { b = 1; }; a.c = 2; }` is `{ a = { b = 1; c = 2; }; }`; only a
// repeated leaf is a duplicate.
func TestAttrPathMerge(t *testing.T) {
	for _, test := range [][2]string{
		{`{ a.b = 1; a.c = 2; }`, `{ a = { b = 1; c = 2; }; }`},
		{`{ a = { b = 1; }; a.c = 2; }`, `{ a = { b = 1; c = 2; }; }`},
		{`{ a = { b = 1; }; a = { c = 2; }; }`, `{ a = { b = 1; c = 2; }; }`},
		{`{ a.b = 1; a = { c = 2; }; }`, `{ a = { b = 1; c = 2; }; }`},
		{`{ a.b.c = 1; a.b.d = 2; }`, `{ a = { b = { c = 1; d = 2; }; }; }`},
		{`{ a.b = { x = 1; }; a.b.c = 2; }`, `{ a = { b = { c = 2; x = 1; }; }; }`},
		{`{ passthru.tests = { a = 1; }; passthru.tests.b = 2; }`,
			`{ passthru = { tests = { a = 1; b = 2; }; }; }`},
		// A recursive set merges the same way, and the merged bindings can
		// still refer to the set being defined.
		{`rec { a = { b = 1; }; a.c = 2; }`, `{ a = { b = 1; c = 2; }; }`},
		{`rec { a = { b = a.c; }; a.c = 2; }.a.b`, `2`},
		{`let a.b = 1; a.c = 2; in a`, `{ b = 1; c = 2; }`},
		// Merging is structural: it must not force a binding that stays unused.
		{`{ a = { b = throw "boom"; }; a.c = 1; }.a.c`, `1`},
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

// TestFunctor covers sets made callable through a __functor attribute: applying
// such a set calls its __functor with the set itself prepended to the arguments.
func TestFunctor(t *testing.T) {
	for _, test := range [][2]string{
		{`{ __functor = self: x: x * 2; } 21`, `42`},
		{`{ __functor = self: x: y: x + y; } 1 2`, `3`},
		// self is the set itself, so it can reach its other attributes.
		{`{ a = 1; __functor = self: x: self.a + x; } 2`, `3`},
		{`rec { a = 2; __functor = self: x: self.a + x; } 1`, `3`},
		{`let f = { a = 10; __functor = self: x: self.a + x; }; in builtins.map f [ 1 2 ]`, `[ 11 12 ]`},
		{`let f = { __functor = self: x: y: x + y; }; in (f 1) 2`, `3`},
		{`let f = { __functor = self: x: x * 2; }; in [ (f 21) (f 3) ]`, `[ 42 6 ]`},
		// Unused attributes of the functor set must stay lazy.
		{`{ a = throw "boom"; __functor = self: x: x; } 1`, `1`},
		// A functor set is still a set, not a lambda.
		{`builtins.typeOf { __functor = self: x: x; }`, `"set"`},
	} {
		got, err := evalPrint(t, test[0])
		if err != nil {
			t.Errorf("%s\nunexpected error: %v", test[0], err)
			continue
		}
		if got != test[1] {
			t.Errorf("%s\n got: %s\nwant: %s", test[0], got, test[1])
		}
	}
}

func TestFunctorErrors(t *testing.T) {
	for _, test := range [][2]string{
		{`{ __functor = 1; } 1`, `value of the __functor attribute is an integer while a function was expected`},
		{`{ __functor = "x"; } 1`, `value of the __functor attribute is a string while a function was expected`},
		{`{ __functor = null; } 1`, `value of the __functor attribute is null while a function was expected`},
		{`{ a = 1; } 1`, `value is a set while a function was expected`},
	} {
		_, err := evalPrint(t, test[0])
		if err == nil {
			t.Errorf("%s\nexpected error %q, got none", test[0], test[1])
			continue
		}
		if !strings.Contains(err.Error(), test[1]) {
			t.Errorf("%s\n got: %v\nwant it to contain: %s", test[0], err, test[1])
		}
	}
}

// TestWithFixpoint checks a fixpoint whose body is `self: with self; …`. The
// with-set is self, so forcing it while looking up a name that a lexical
// binding provides — or before any lookup at all — recurses. Both the set and
// the lookup must stay lazy.
func TestWithFixpoint(t *testing.T) {
	for _, test := range [][2]string{
		// A lexical binding beats the with-set without forcing it.
		{`let x = 1; f = self: with self; { a = x; }; self = f self; in self.a`, `1`},
		// A name read straight out of the recursive set through `with self`.
		{`let f = self: with self; { a = 1; b = a; }; self = f self; in self.b`, `1`},
		{`let f = self: with self; { b = a; a = 2; }; self = f self; in self.b`, `2`},
	} {
		got, err := evalPrint(t, test[0])
		if err != nil {
			t.Errorf("%s\nunexpected error: %v", test[0], err)
			continue
		}
		if got != test[1] {
			t.Errorf("%s\n got: %s\nwant: %s", test[0], got, test[1])
		}
	}
}

// TestMakeScopeFixpoint reproduces lib.makeScope, whose fixpoint body is
// `self: with self; …`. The makeScope argument — a name resolved lexically —
// must be found without forcing self, or the whole fixpoint recurses.
func TestMakeScopeFixpoint(t *testing.T) {
	src := `let
	  makeScope = newScope: f:
	    let
	      self = { callPackage = self.newScope { }; } // f self // {
	        newScope = scope: newScope (self // scope);
	      };
	    in self;
	  hostPlatform = "linux";
	in (makeScope null (self: with self; { result = hostPlatform; })).result`
	got, err := evalPrint(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"linux"`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// TestErrorTrace checks that an error carries a position and the chain of
// evaluations that led to it.
func TestErrorTrace(t *testing.T) {
	_, err := EvalString(`let f = x: x.missing; in { a.b = f { }; }.a.b`)
	if err == nil {
		t.Fatal("expected an error")
	}
	e, ok := err.(*EvalError)
	if !ok {
		t.Fatalf("expected an *EvalError, got %T", err)
	}
	if e.Kind != ErrMissingAttribute {
		t.Errorf("kind = %v, want ErrMissingAttribute", e.Kind)
	}
	if e.Pos == nil {
		t.Fatal("error carries no position")
	}
	if e.Pos.Column != 12 { // the `x.missing` selection
		t.Errorf("column = %d, want 12", e.Pos.Column)
	}
	rendered := e.Error()
	for _, want := range []string{
		"error: attribute 'missing' missing",
		"let f = x: x.missing; in { a.b = f { }; }.a.b",
		"while calling a function",
		"while evaluating the attribute 'b'",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered error is missing %q:\n%s", want, rendered)
		}
	}
}

// TestInfiniteRecursionTerminates makes sure a cycle is reported rather than
// exhausting the stack, and that the backtrace stays bounded.
func TestInfiniteRecursionTerminates(t *testing.T) {
	_, err := EvalString(`let f = x: f x; in f 1`)
	if err == nil {
		t.Fatal("expected an error")
	}
	e, ok := err.(*EvalError)
	if !ok {
		t.Fatalf("expected an *EvalError, got %T", err)
	}
	if len(e.Trace) > maxTraceFrames {
		t.Errorf("trace has %d frames, want at most %d", len(e.Trace), maxTraceFrames)
	}
}

// TestSharedPartialApplication guards against a partially applied builtin
// leaking arguments between its uses.
func TestSharedPartialApplication(t *testing.T) {
	got, err := evalPrint(t, `let f = builtins.add 1; in [ (f 10) (f 20) ]`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[ 11 21 ]`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func BenchmarkEval(b *testing.B) {
	src := `let
	  range = n: builtins.genList (i: i) n;
	  sum = builtins.foldl' (a: x: a + x) 0;
	  double = map (x: x * 2);
	in sum (double (range 200))`
	for i := 0; i < b.N; i++ {
		if _, err := EvalString(src); err != nil {
			b.Fatal(err)
		}
	}
}

// TestStringLiterals covers escape processing and the indentation stripping of
// indented strings.
func TestStringLiterals(t *testing.T) {
	for _, test := range [][2]string{
		{`"a\nb"`, "a\nb"},
		{`"a\tb"`, "a\tb"},
		{`"a\"b"`, `a"b`},
		{`"a\\b"`, `a\b`},
		{`"a\${b}"`, `a${b}`},
		{"''foo''", "foo"},
		{"''\n  foo\n    bar\n  baz\n''", "foo\n  bar\nbaz\n"},
		{"''\n  foo\n\n  bar\n''", "foo\n\nbar\n"},
		{"''\n  a${\"b\"}c\n''", "abc\n"},
		{"''\n  ''${literal}\n''", "${literal}\n"},
		{"''\n  a''\\nb\n''", "a\nb\n"},
		// The indentation of the closing quotes must not count as common
		// indentation.
		{"''\n    deep\n  ''", "deep\n"},
	} {
		val, err := EvalString(test[0])
		if err != nil {
			t.Errorf("%s\nunexpected error: %v", test[0], err)
			continue
		}
		if val.Kind() != KindString {
			t.Errorf("%s: got %s, want a string", test[0], TypeName(val))
			continue
		}
		if got := val.Str().Content; got != test[1] {
			t.Errorf("%s\n got: %q\nwant: %q", test[0], got, test[1])
		}
	}
}

// TestPrintDepth checks that printing abbreviates below the requested depth,
// which is what keeps printing a self-referential value finite.
func TestPrintDepth(t *testing.T) {
	val, err := EvalString(`{ a = { b = { c = 1; }; }; l = [ [ 1 ] ]; }`)
	if err != nil {
		t.Fatal(err)
	}
	for depth, want := range map[int]string{
		0: `{ ... }`,
		1: `{ a = { ... }; l = [ ... ]; }`,
		2: `{ a = { b = { ... }; }; l = [ [ ... ] ]; }`,
		3: `{ a = { b = { c = 1; }; }; l = [ [ 1 ] ]; }`,
	} {
		got, err := Print(val, depth)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("depth %d:\n got: %s\nwant: %s", depth, got, want)
		}
	}
}

// TestPrintErrorIsCaught makes sure a failure that only happens while forcing
// nested values is reported rather than escaping as a panic.
func TestPrintErrorIsCaught(t *testing.T) {
	val, err := EvalString(`{ a = throw "boom"; }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Print(val, 2); err == nil {
		t.Fatal("expected printing to fail")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("got %v, want it to mention boom", err)
	}
}
