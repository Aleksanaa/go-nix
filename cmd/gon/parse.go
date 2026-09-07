package main

import (
	"fmt"

	"github.com/alecthomas/kingpin"
	"github.com/orivej/go-nix/pkg/parser"
)

var (
	parseCmd     = kingpin.Command("parse", "Parse Nix expression.")
	parseExprArg = parseCmd.Arg("expr", "Expression.").Required().String()
	parseFile    = parseCmd.Flag("file", "Parse file.").Short('f').Bool()
)

var parseMain = register("parse", func() {
	parse := parser.ParseString
	if *parseFile {
		parse = parser.ParseFile
	}
	pr, err := parse(*parseExprArg)
	fail(err)
	fmt.Println(pr.LispResult())
})
