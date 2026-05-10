package isa

// Format identifies which of the six RV32I encoding shapes an instruction uses.
type Format int

const (
	FmtR Format = iota
	FmtI
	FmtIShift // I-type with funct7 in shift-amount position (slli/srli/srai)
	FmtS
	FmtB
	FmtU
	FmtJ
	FmtSys // system: ecall, ebreak (encoded as I-type with fixed imm)
	FmtFence
)

// OpDef describes a single instruction mnemonic.
type OpDef struct {
	Name   string
	Fmt    Format
	Opcode uint32
	Funct3 uint32
	Funct7 uint32 // also used as fixed imm[11:5] for shifts
}

// Opcodes — looked up by mnemonic. Sourced from RISC-V User-Level ISA Manual,
// Volume I, Chapter 19 (instruction listing).
var Opcodes = map[string]OpDef{
	// --- R-type ALU ---------------------------------------------------------
	"add":  {"add", FmtR, 0x33, 0x0, 0x00},
	"sub":  {"sub", FmtR, 0x33, 0x0, 0x20},
	"sll":  {"sll", FmtR, 0x33, 0x1, 0x00},
	"slt":  {"slt", FmtR, 0x33, 0x2, 0x00},
	"sltu": {"sltu", FmtR, 0x33, 0x3, 0x00},
	"xor":  {"xor", FmtR, 0x33, 0x4, 0x00},
	"srl":  {"srl", FmtR, 0x33, 0x5, 0x00},
	"sra":  {"sra", FmtR, 0x33, 0x5, 0x20},
	"or":   {"or", FmtR, 0x33, 0x6, 0x00},
	"and":  {"and", FmtR, 0x33, 0x7, 0x00},

	// --- I-type ALU ---------------------------------------------------------
	"addi":  {"addi", FmtI, 0x13, 0x0, 0x00},
	"slti":  {"slti", FmtI, 0x13, 0x2, 0x00},
	"sltiu": {"sltiu", FmtI, 0x13, 0x3, 0x00},
	"xori":  {"xori", FmtI, 0x13, 0x4, 0x00},
	"ori":   {"ori", FmtI, 0x13, 0x6, 0x00},
	"andi":  {"andi", FmtI, 0x13, 0x7, 0x00},
	// I-shift uses funct7 as the high seven bits of imm[11:5]
	"slli": {"slli", FmtIShift, 0x13, 0x1, 0x00},
	"srli": {"srli", FmtIShift, 0x13, 0x5, 0x00},
	"srai": {"srai", FmtIShift, 0x13, 0x5, 0x20},

	// --- Loads (I-type) -----------------------------------------------------
	"lb":  {"lb", FmtI, 0x03, 0x0, 0x00},
	"lh":  {"lh", FmtI, 0x03, 0x1, 0x00},
	"lw":  {"lw", FmtI, 0x03, 0x2, 0x00},
	"lbu": {"lbu", FmtI, 0x03, 0x4, 0x00},
	"lhu": {"lhu", FmtI, 0x03, 0x5, 0x00},

	// --- Stores (S-type) ----------------------------------------------------
	"sb": {"sb", FmtS, 0x23, 0x0, 0x00},
	"sh": {"sh", FmtS, 0x23, 0x1, 0x00},
	"sw": {"sw", FmtS, 0x23, 0x2, 0x00},

	// --- Branches (B-type) --------------------------------------------------
	"beq":  {"beq", FmtB, 0x63, 0x0, 0x00},
	"bne":  {"bne", FmtB, 0x63, 0x1, 0x00},
	"blt":  {"blt", FmtB, 0x63, 0x4, 0x00},
	"bge":  {"bge", FmtB, 0x63, 0x5, 0x00},
	"bltu": {"bltu", FmtB, 0x63, 0x6, 0x00},
	"bgeu": {"bgeu", FmtB, 0x63, 0x7, 0x00},

	// --- Jumps --------------------------------------------------------------
	"jal":  {"jal", FmtJ, 0x6F, 0x0, 0x00},
	"jalr": {"jalr", FmtI, 0x67, 0x0, 0x00},

	// --- Upper-immediate (U-type) ------------------------------------------
	"lui":   {"lui", FmtU, 0x37, 0x0, 0x00},
	"auipc": {"auipc", FmtU, 0x17, 0x0, 0x00},

	// --- System -------------------------------------------------------------
	"ecall":  {"ecall", FmtSys, 0x73, 0x0, 0x00},
	"ebreak": {"ebreak", FmtSys, 0x73, 0x0, 0x00},

	// --- Fence (treated as a fixed-encoding nop-ish for our SoC) -----------
	"fence": {"fence", FmtFence, 0x0F, 0x0, 0x00},
}

// LookupOp returns (def, true) if the mnemonic is recognized.
func LookupOp(name string) (OpDef, bool) {
	d, ok := Opcodes[name]
	return d, ok
}
