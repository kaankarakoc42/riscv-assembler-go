// rvld — link multiple RVOB1 objects into a flat memory image.
//
// Usage:
//
//	rvld -o out -script link.toml file1.ro file2.ro …
//
// Produces:
//
//	out.bin  — raw little-endian bytes
//	out.hex  — Intel HEX
//	out.mem  — $readmemh-compatible word-per-line file
//	out.map  — human-readable section/symbol map
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/linker"
	"github.com/edu/rvtoolchain/linker/image"
	rl "github.com/edu/rvtoolchain/linker/reloc"
	"github.com/edu/rvtoolchain/linker/script"
)

func main() {
	var out, scriptPath string
	flag.StringVar(&out, "o", "a.out", "output base name (without extension)")
	flag.StringVar(&scriptPath, "script", "", "linker script (default: built-in PicoRV32 layout)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: rvld -o OUTBASE [-script LINK.toml] file1.ro file2.ro ...\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(scriptPath, out, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "rvld: %v\n", err)
		os.Exit(1)
	}
}

func run(scriptPath, outBase string, files []string) error {
	// 1. Parse linker script.
	var sc *script.Script
	if scriptPath == "" {
		sc = script.Default()
	} else {
		raw, err := os.ReadFile(scriptPath)
		if err != nil {
			return fmt.Errorf("read script: %w", err)
		}
		sc, err = script.Parse(raw)
		if err != nil {
			return err
		}
	}

	// 2. Load all input objects.
	inputs := make([]linker.Input, 0, len(files))
	for _, p := range files {
		raw, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		mod, err := obj.Read(raw)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		inputs = append(inputs, linker.Input{Path: p, Module: mod})
	}

	// 3. Layout + symbol resolution.
	img, err := linker.Link(inputs, sc)
	if err != nil {
		return err
	}

	// 4. Apply relocations.
	if err := rl.Apply(img); err != nil {
		return err
	}

	// 5. Emit outputs.
	if err := image.EmitAll(img, outBase); err != nil {
		return err
	}

	// Bonus: a human-readable .map file.
	if err := emitMap(outBase+".map", img); err != nil {
		return err
	}

	return nil
}

func emitMap(path string, img *linker.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "Memory layout:\n")
	fmt.Fprintf(f, "  .text @ 0x%08X (%d bytes)\n", img.TextBase, len(img.TextData))
	fmt.Fprintf(f, "  .data @ 0x%08X (%d bytes)\n", img.DataBase, len(img.DataData))
	fmt.Fprintf(f, "\nInput placements:\n")
	for _, p := range img.Placements {
		fmt.Fprintf(f, "  %-40s %s @ 0x%08X (%d bytes)\n",
			img.Inputs[p.InputIdx].Path, p.Name, p.Base, p.Size)
	}
	fmt.Fprintf(f, "\nGlobal symbols:\n")
	names := make([]string, 0, len(img.Globals))
	for n := range img.Globals {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		g := img.Globals[n]
		fmt.Fprintf(f, "  %-32s 0x%08X  (from %s)\n", n, g.Address, img.Inputs[g.Defining].Path)
	}
	return nil
}
