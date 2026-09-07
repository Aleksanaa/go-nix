package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/aleksanaa/go-nix/pkg/eval"
)

var replCmd = kingpin.Command("repl", "Read, evaluate and print loop.")

var replMain = register("repl", func() {
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for {
		fmt.Fprint(out, "nix-repl> ")
		out.Flush()
		if !in.Scan() {
			fmt.Fprintln(out)
			break
		}
		line := strings.TrimSpace(in.Text())
		switch line {
		case "":
			continue
		case ":q", ":quit":
			return
		}
		// An error in one entry must not end the session, so it is reported
		// and the loop continues.
		val, err := eval.EvalString(line)
		if err == nil {
			var s string
			if s, err = eval.Print(val, 1); err == nil {
				fmt.Fprintln(out, s)
				continue
			}
		}
		fmt.Fprintln(out, err)
	}
	if err := in.Err(); err != nil && err != io.EOF {
		fail(err)
	}
})
