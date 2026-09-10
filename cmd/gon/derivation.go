package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/aleksanaa/go-nix/pkg/eval"
	"github.com/aleksanaa/go-nix/pkg/parser"
)

var (
	derivationCmd  = kingpin.Command("derivation", "Show the derivation a Nix expression evaluates to.")
	derivationExpr = derivationCmd.Arg("expr", "Expression.").Required().String()
	derivationFile = derivationCmd.Flag("file", "Evaluate a file.").Short('f').Bool()
)

var derivationMain = register("derivation", func() {
	parse := parser.ParseString
	if *derivationFile {
		parse = parser.ParseFile
	}
	pr, err := parse(*derivationExpr)
	fail(err)
	val, err := eval.Eval(pr)
	fail(err)

	d := eval.DerivationOf(val)
	if d == nil {
		fail(fmt.Errorf("expression did not evaluate to a derivation"))
	}

	body, err := d.JSON()
	fail(err)
	// The derivation path is named by its store-path name, as `nix
	// derivation show` does.
	name := strings.TrimPrefix(d.DrvPath(), "/nix/store/")
	out := map[string]any{
		"version": 4,
		"derivations": map[string]json.RawMessage{
			name: body,
		},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	fail(enc.Encode(out))
	fmt.Print(buf.String())
})
