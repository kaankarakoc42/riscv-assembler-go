// Package reloc defines the set of relocation types our linker understands.
//
// A relocation is a "fix me later" note left in the object file:
//
//   "At byte offset O of section S, splice the value of symbol Y
//    (or Y - PC) into bits B..A of the 32-bit word at that offset,
//    using the encoding scheme T."
//
// The linker walks every relocation after it has decided where each section
// (and therefore each symbol) lives in the final memory image, computes the
// patched value, and writes it back into the section bytes.
package reloc

// Type is the relocation kind.
//
// Naming convention follows official RISC-V ELF psABI ("R_RISCV_*") so that
// students moving on to real toolchains have a head start. We use a tighter
// numeric range here.
type Type uint8

const (
	// R_RV32_NONE is a no-op marker. Useful as a sentinel.
	R_RV32_NONE Type = 0

	// R_RV32_32 — absolute 32-bit value at the patch site.
	// Used by ".word symbol".
	R_RV32_32 Type = 1

	// R_RV32_BRANCH — B-type, 13-bit signed PC-relative, imm[0]=0.
	// Patches: imm[12]→bit31, imm[10:5]→bits30:25, imm[4:1]→bits11:8,
	//          imm[11]→bit7. Range: ±4 KiB.
	R_RV32_BRANCH Type = 2

	// R_RV32_JAL — J-type, 21-bit signed PC-relative, imm[0]=0.
	// Patches the imm field of a jal instruction. Range: ±1 MiB.
	R_RV32_JAL Type = 3

	// R_RV32_HI20 — sets the upper 20 bits of an absolute 32-bit address
	// (with rounding for the LO12 sign-extension). Used by lui.
	R_RV32_HI20 Type = 4

	// R_RV32_LO12_I — fills the I-type immediate slot with the low 12 bits
	// of an absolute address. Used after lui by addi/lw/etc.
	R_RV32_LO12_I Type = 5

	// R_RV32_LO12_S — same as LO12_I but for S-type encoding (sw/sb/sh).
	R_RV32_LO12_S Type = 6

	// R_RV32_PCREL_HI20 — auipc upper 20 bits, PC-relative.
	R_RV32_PCREL_HI20 Type = 7

	// R_RV32_PCREL_LO12_I — paired with PCREL_HI20: encodes the low 12 bits
	// of (symbol - PC_of_paired_auipc). The "Addend" field carries the offset
	// from the LO12 site back to the matching auipc, in the same input
	// section. This matches the GNU psABI convention.
	R_RV32_PCREL_LO12_I Type = 8
)

// String returns a short, human-readable name for tooling output.
func (t Type) String() string {
	switch t {
	case R_RV32_NONE:
		return "R_RV32_NONE"
	case R_RV32_32:
		return "R_RV32_32"
	case R_RV32_BRANCH:
		return "R_RV32_BRANCH"
	case R_RV32_JAL:
		return "R_RV32_JAL"
	case R_RV32_HI20:
		return "R_RV32_HI20"
	case R_RV32_LO12_I:
		return "R_RV32_LO12_I"
	case R_RV32_LO12_S:
		return "R_RV32_LO12_S"
	case R_RV32_PCREL_HI20:
		return "R_RV32_PCREL_HI20"
	case R_RV32_PCREL_LO12_I:
		return "R_RV32_PCREL_LO12_I"
	}
	return "R_RV32_?"
}
