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
