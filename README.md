[![Build Status](https://travis-ci.org/orivej/go-nix.svg?branch=master)](https://travis-ci.org/orivej/go-nix)

# Overview

This repository contains:

- `pkg/parser` — a Nix parser. Optimized for speed, it parses all Nixpkgs in 2 seconds. It preserves comments and source positions and can be used to implement Nix files formatting.
- `pkg/eval` — a lazy evaluator for the Nix language. It is incomplete (no derivations, no imports, no store), but it evaluates the language itself, and reports failures the way Nix does.
- `pkg/nixhash` — Nix-compatible hasher for store paths.
- `cmd/gon` — an utility that exposes these libraries from the command line.

# The evaluator

## Layout

| File | Contents |
| --- | --- |
| `expr.go` | `Expression`, the thunk every expression is evaluated through |
| `scope.go` | identifier lookup, including the low-priority scopes `with` introduces |
| `eval.go` | evaluation of each kind of syntax node |
| `eval_op.go` | the operators |
| `error.go` | `EvalError`, error kinds, and backtrace capture |
| `value.go`, `number.go`, `string.go`, `path.go`, `list.go`, `set.go`, `lambda.go` | the value types |
| `coerce.go`, `strlit.go` | string coercion, and string literal escapes and indentation |
| `builtins*.go` | the `builtins` set, grouped by the type it works on |

## Laziness

Every expression is an `Expression`: a thunk holding the syntax node, the scope
it closes over, and the value once it has been forced. Forcing is memoized, and
a thunk that delegates to another (a `let` body, a function call, a selected
attribute) caches the result too, so a chain is only walked once.

Nothing is evaluated that the result does not depend on, which the test suite
pins down with expressions such as `(a: 1) (throw "boom")` and
`{ a = throw "boom"; } ? a`.

## Errors

Failures are raised as panics carrying an `*EvalError` and turned back into
ordinary Go errors at the package boundary, by `Eval`, `EvalString` and `Print`.
Each error records what went wrong, where, and the chain of evaluations that
led there:

```
$ gon eval -f example.nix
error: value is a set while a list was expected

       at example.nix:14:27:
           14|   lib.mapAttrs (name: v: builtins.length v) config
             |                           ^

       … while calling the 'length' builtin

       … while calling a function
         at example.nix:14:24:
             14|   lib.mapAttrs (name: v: builtins.length v) config
               |                        ^

       … while evaluating the attribute 'value'
         at example.nix:6:18:
              6|         value = f name set.${name};
               |                  ^
```

The backtrace is read off the stack of expressions being evaluated at the
moment of failure, so unwinding stays a single panic; annotating each frame on
the way out would be quadratic. Only frames worth naming are kept, and their
number is capped.

`ErrorKind` says what kind of failure it was, and decides what
`builtins.tryEval` catches: assertions and `throw`, but not type errors or
`abort`, matching Nix.

Non-termination is reported rather than crashing: a thunk that is already being
forced fails with `infinite recursion encountered`, and evaluation that nests
past `maxCallDepth` fails with a stack overflow error.

# Examples

`examples/hanoi.nix` solves the Towers of Hanoi, and `examples/hanoi-calls.nix`
only counts the moves; both evaluate to the same value under `nix` and under
`gon`, and the work doubles with every disk added.

```sh
$ gon eval -f examples/hanoi.nix
$ nix-instantiate --eval examples/hanoi.nix
$ examples/bench.sh hanoi.nix 10 18   # time both, checking they agree
```

## Where the time goes

Evaluation is allocation bound: a CPU profile (`gon --profile eval -f ...`,
then `go tool pprof`) spends more than half its samples in the garbage
collector, and an allocation profile names the thunks and the scopes rather
than any computation. Four changes came out of that, on 18-disk Hanoi:

| | time | peak RSS |
| --- | --- | --- |
| starting point | 1556 ms | 761 MB |
| 64-byte thunks, and forced thunks dropping what produced them | 1214 ms | 608 MB |
| operands evaluated without allocating a thunk | 1155 ms | 608 MB |
| single-argument calls binding without a map | 882 ms | 311 MB |
| Nix, for comparison | 139 ms | |

The first two are the interesting ones. A thunk that has been forced is
nothing but its value, so `force` clears its scope, node and delegate; keeping
them alive was pinning whole scope chains for as long as any value derived
from them lived. And an operand that is consumed immediately — the two sides
of an operator, a condition, the function of an application — never needs to
become a thunk at all: the evaluation stack records copies rather than
pointers, so those expressions stay on the Go stack.

What remains is structural, and the profile is unambiguous about the order:
function calls allocate a Go map per call frame when the function has formal
arguments, every expression that is genuinely lazy still costs a heap thunk,
and attribute sets are Go maps where Nix uses sorted arrays. Closing the
remaining gap means what Nix does: a pass over the AST that resolves each
variable to a position in a flat environment, values as a tagged struct rather
than a Go interface (which boxes every integer), and evaluation into a
caller-provided value instead of an allocated thunk.

`exprSlabSize` in `expr.go` allocates thunks in blocks. It is off by default:
it is 14% faster and 2.4 times larger, because a block cannot be freed until
every thunk in it is unreachable.

# Development

## How to build

```sh
$ nix build
```

## Run cmd/gon

```sh
$ result/bin/gon --help
$ result/bin/gon eval '{ a = 1; } // { b = 2; }'
$ result/bin/gon eval -f example.nix --strict
$ result/bin/gon parse '{ a = 1; }'
$ result/bin/gon repl
```

## Get a development environment

```sh
$ nix develop
$ go build ./...
```

## Run tests

```sh
$ go test ./...
```

The parser tests evaluate all of Nixpkgs when `NIX_PATH` points at it, and skip
themselves otherwise. Use `go test -short ./...` to skip them regardless.

# Credits

- [ragel](https://www.colm.net/open-source/ragel/) generates the Nix lexer
- [goyacc](https://godoc.org/golang.org/x/tools/cmd/goyacc) generates the Nix parser
- [kingpin](https://github.com/alecthomas/kingpin) powers the CLI
