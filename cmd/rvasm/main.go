// rvasm — assemble one RV32I source file into a relocatable RVOB1 object.
//
// Usage:
//
//	rvasm -o out.ro input.s
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/edu/rvtoolchain/assembler/encoder"
	"github.com/edu/rvtoolchain/assembler/lexer"
	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/obj"
)

func main() {
	var out string
	flag.StringVar(&out, "o", "", "output object file (.ro)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rvasm -o out.ro input.s\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	in := flag.Arg(0)
	if out == "" {
		out = trimExt(in) + ".ro"
	}

	if err := run(in, out); err != nil {
		fmt.Fprintf(os.Stderr, "rvasm: %v\n", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string) error {
	src, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}

	toks, err := lexer.Lex(inPath, src)
	if err != nil {
		return err
	}

	stmts, err := parser.Parse(inPath, toks)
	if err != nil {
		return err
	}

	mod, err := encoder.Encode(stmts)
	if err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return obj.Write(f, mod)
}

func trimExt(p string) string {
	for i := len(p) - 1; i >= 0 && p[i] != '/' && p[i] != '\\'; i-- {
		if p[i] == '.' {
			return p[:i]
		}
	}
	return p
}
