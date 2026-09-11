[![Build Status](https://travis-ci.org/orivej/go-nix.svg?branch=master)](https://travis-ci.org/orivej/go-nix)

# Overview

This repository contains:

- `pkg/parser` — a Nix parser. Optimized for speed, it parses all Nixpkgs in 2 seconds. It preserves comments and source positions and can be used to implement Nix files formatting.
- `pkg/eval` — a lazy evaluator for the Nix language. It evaluates the language itself, reports failures the way Nix does, builds and hashes derivations in memory, and loads files through `import` and the source builtins (`readFile`, `readDir`, `builtins.path`, …); no store is written, which is Nix's read-only mode.
- `pkg/nixhash` — Nix-compatible hasher for store paths and derivations.
- `pkg/source` — the evaluator's access to the file system: resolving paths and reading files and directories.
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

`examples/` holds a benchmark suite: self-contained Nix expressions that both
`nix` and `gon` accept, each one leaning on a different part of an evaluator.
Every file takes its size from a single `n` on its own line, so a workload can
be scaled without editing it.

| workload | what it leans on |
| --- | --- |
| `hanoi-calls` | function application and integer arithmetic, and nothing else |
| `hanoi` | list building and set construction, doubling with every disk |
| `attrs` | building, merging and indexing large attribute sets |
| `lookup` | resolving names ten scopes up, and through a `with` |
| `lists` | `map`, `filter`, `sort` and a strict fold calling back into Nix |
| `strings` | interpolation, concatenation and the string builtins |
| `lazy` | creating three million thunks and forcing thirty thousand |
| `fix` | a fixpoint with an overlay, the shape Nixpkgs is built from |
| `set-force` | forcing every value of a wide set, the shape of a dependency closure |

```sh
$ gon eval -f examples/hanoi.nix
$ nix-instantiate --eval examples/hanoi.nix
$ examples/bench.sh                   # the whole suite
$ examples/bench.sh hanoi.nix 10 18   # one workload, swept over n
```

`bench.sh` checks that the two evaluators produce the same value before it
reports a timing, and subtracts each one's startup from the ratio.

## Where the time goes

Across the suite gon is 2.8 to 5.5 times slower than Nix. The spread is the
interesting part: it is smallest on pure function calls and arithmetic
(`hanoi-calls`, 2.8x) and largest where values are built rather than computed
(`hanoi`, 5.5x).

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
| delegating to a sub-expression without a thunk | 815 ms | 311 MB |
| literals and names worked out once per node, not per evaluation | 745 ms | 300 MB |
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

`exprSlabSize` and `scopeSlabSize` allocate thunks and scopes in blocks. A
block cannot be freed until every member in it is unreachable, but the
benchmark workloads' lifetimes line up, so blocks of 2048 thunks and 1024
scopes measure ~5% faster than blocks of 256 for less total allocation and no
more live heap.

## Borrowed from the TypeScript compiler

The Go port of the TypeScript compiler solves two of the same problems, and
`static.go` follows it:

- Nodes are numbered densely by the parser, so anything worked out about a node
  is kept in a sparse array of fixed-size pages rather than a map — two array
  indexes and no hashing. Their version is `core.PagedLinkStore`. The `ID` field
  this needs packs into padding `parser.Node` already had, so it is free.
- Their `core.Arena` is the same block allocator as `exprSlabSize`, and they
  keep it on, because a compiler holds its whole syntax and type graph for the
  session. That is the rule this evaluator fails: it allocates values whose
  lifetimes are mixed, so a block outlives most of what is in it.

Their comment that "an interface call is opaque to escape analysis" is also
worth keeping in mind here: it is why evaluating an operand through a concrete
type keeps it off the heap, and it is an argument against `NixValue` being an
interface at all.

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
$ result/bin/gon derivation -f example.nix   # derivation JSON, like `nix derivation show`
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
- all [Nix](https://github.com/NixOS/nix) contributors, whose implementation this
  project follows for behaviour

# License

go-nix is licensed under the GNU Lesser General Public License, version 2.1 or
later (LGPL-2.1-or-later). See [LICENSE](LICENSE).

It is a derivative of [Orivej Desh's go-nix](https://github.com/orivej/go-nix),
released into the public domain under the [UNLICENSE](UNLICENSE); in particular
`pkg/parser`, `pkg/nixhash`, `pkg/util` and `cmd/gon` are based on that work.
To keep behaviour identical, the project was written with heavy reference to the
[official Nix codebase](https://github.com/NixOS/nix), which is itself
LGPL-2.1-or-later. See [NOTICE](NOTICE) for the full attribution.

