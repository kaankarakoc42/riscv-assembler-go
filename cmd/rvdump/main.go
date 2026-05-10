// rvdump — pretty-print an RVOB1 object file.
//
// Usage:
//
//	rvdump file.ro [file2.ro ...]
//
// For each file, prints:
//
//   - section table
//   - symbol table
//   - relocation table
//   - hex+disassembly of .text
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"

	"github.com/edu/rvtoolchain/common/obj"
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: rvdump file.ro [file2.ro ...]")
		os.Exit(2)
	}
	for i, p := range flag.Args() {
		if i > 0 {
			fmt.Println()
		}
		if err := dump(p); err != nil {
			fmt.Fprintf(os.Stderr, "rvdump: %s: %v\n", p, err)
			os.Exit(1)
		}
	}
}

func dump(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m, err := obj.Read(raw)
	if err != nil {
		return err
	}

	fmt.Printf("=== %s ===\n", path)

	fmt.Println("\nSections:")
	fmt.Printf("  %-3s %-12s %5s  %s\n", "idx", "name", "size", "flags")
	for i, s := range m.Sections {
		fmt.Printf("  %-3d %-12s %5d  %s\n", i, s.Name, len(s.Data), formatFlags(s.Flags))
	}

	if len(m.Symbols) > 0 {
		fmt.Println("\nSymbols:")
		fmt.Printf("  %-3s %-24s %-7s %-3s %s\n", "idx", "name", "bind", "sec", "value")
		for i, s := range m.Symbols {
			sec := fmt.Sprintf("%d", s.SectionIdx)
			if s.SectionIdx == obj.SectionExtern {
				sec = "ext"
			}
			fmt.Printf("  %-3d %-24s %-7s %-3s 0x%08X\n", i, s.Name, s.Bind, sec, s.Value)
		}
	}

	if len(m.Relocs) > 0 {
		fmt.Println("\nRelocations:")
		fmt.Printf("  %-3s %-3s %-10s %-22s %-8s %s\n", "sec", "off", "type", "symbol", "addend", "")
		for _, r := range m.Relocs {
			sym := "<bad>"
			if int(r.SymIdx) < len(m.Symbols) {
				sym = m.Symbols[r.SymIdx].Name
			}
			fmt.Printf("  %-3d %3d  %-10s %-22s %-8d\n", r.SectionIdx, r.Offset, r.Type, sym, r.Addend)
		}
	}

	textIdx := m.FindSection(".text")
	if textIdx >= 0 && len(m.Sections[textIdx].Data) > 0 {
		fmt.Println("\n.text:")
		dumpHex(m.Sections[textIdx].Data, 0)
	}
	dataIdx := m.FindSection(".data")
	if dataIdx >= 0 && len(m.Sections[dataIdx].Data) > 0 {
		fmt.Println("\n.data:")
		dumpHexBytes(m.Sections[dataIdx].Data, 0)
	}
	return nil
}

func formatFlags(f uint32) string {
	s := ""
	if f&obj.FlagAlloc != 0 {
		s += "A"
	}
	if f&obj.FlagExec != 0 {
		s += "X"
	}
	if f&obj.FlagWrite != 0 {
		s += "W"
	}
	return s
}

func dumpHex(data []byte, base uint32) {
	for i := 0; i+4 <= len(data); i += 4 {
		w := binary.LittleEndian.Uint32(data[i:])
		fmt.Printf("  %08X: %08x\n", base+uint32(i), w)
	}
}

func dumpHexBytes(data []byte, base uint32) {
	for i := 0; i < len(data); i += 16 {
		fmt.Printf("  %08X:", base+uint32(i))
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		for j := i; j < end; j++ {
			fmt.Printf(" %02X", data[j])
		}
		fmt.Println()
	}
}
