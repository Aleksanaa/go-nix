package main

import (
	"fmt"

	"github.com/alecthomas/kingpin"
	"github.com/orivej/go-nix/pkg/eval"
	"github.com/orivej/go-nix/pkg/parser"
)

var (
	evalCmd     = kingpin.Command("eval", "Eval Nix expression.")
	evalExprArg = evalCmd.Arg("expr", "Expression.").Required().String()
	evalFile    = evalCmd.Flag("file", "Evaluate a file.").Short('f').Bool()
	evalDepth   = evalCmd.Flag("depth", "How deep to print nested lists and sets.").Default("1").Int()
	evalStrict  = evalCmd.Flag("strict", "Evaluate the result completely.").Bool()
)

var evalMain = register("eval", func() {
	parse := parser.ParseString
	if *evalFile {
		parse = parser.ParseFile
	}
	pr, err := parse(*evalExprArg)
	fail(err)
	val, err := eval.Eval(pr)
	fail(err)
	depth := *evalDepth
	if *evalStrict {
		depth = -1 // negative never reaches zero, so nothing is abbreviated
	}
	// Printing forces the value, so it can fail just as evaluating it can.
	s, err := eval.Print(val, depth)
	fail(err)
	fmt.Println(s)
})
