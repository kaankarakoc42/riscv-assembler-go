// Package isa provides RV32I encoding tables and helpers shared by the
// assembler, the disassembler (rvdump), and the relocation engine in the
// linker.
//
// The ISA package is intentionally dependency-free. Anyone who wants to
// understand the toolchain can read it without touching parser glue.
package isa

import "fmt"

// Register names. The RISC-V ABI defines two parallel naming conventions:
// architectural (x0..x31) and ABI (zero, ra, sp, gp, tp, t0..t6, s0..s11,
// a0..a7). Both are accepted by the assembler.
//
// We keep a single forward map (name -> number) and validate at parse time.
var registerNames = map[string]uint32{
	"x0": 0, "x1": 1, "x2": 2, "x3": 3, "x4": 4, "x5": 5, "x6": 6, "x7": 7,
	"x8": 8, "x9": 9, "x10": 10, "x11": 11, "x12": 12, "x13": 13, "x14": 14, "x15": 15,
	"x16": 16, "x17": 17, "x18": 18, "x19": 19, "x20": 20, "x21": 21, "x22": 22, "x23": 23,
	"x24": 24, "x25": 25, "x26": 26, "x27": 27, "x28": 28, "x29": 29, "x30": 30, "x31": 31,

	"zero": 0, "ra": 1, "sp": 2, "gp": 3, "tp": 4,
	"t0": 5, "t1": 6, "t2": 7,
	"s0": 8, "fp": 8, "s1": 9,
	"a0": 10, "a1": 11, "a2": 12, "a3": 13, "a4": 14, "a5": 15, "a6": 16, "a7": 17,
	"s2": 18, "s3": 19, "s4": 20, "s5": 21, "s6": 22, "s7": 23, "s8": 24, "s9": 25, "s10": 26, "s11": 27,
	"t3": 28, "t4": 29, "t5": 30, "t6": 31,
}

// Reg looks up a register by either name or ABI alias. Case-insensitive.
// Returns (number, true) on success.
func Reg(name string) (uint32, bool) {
	// Lowercase manually to avoid pulling in unicode/strings dependency tax.
	lower := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lower[i] = c
	}
	n, ok := registerNames[string(lower)]
	return n, ok
}

// MustReg is the panicking variant; intended for compile-time tables.
func MustReg(name string) uint32 {
	r, ok := Reg(name)
	if !ok {
		panic(fmt.Sprintf("isa: unknown register %q", name))
	}
	return r
}
