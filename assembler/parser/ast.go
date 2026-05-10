// Package parser turns a stream of lexer tokens into a flat list of statements.
package parser

// Operand is a single operand of an instruction or directive.
//
// Five kinds, distinguished by which field is populated:
//
//   - register   ("x5", "t0")   → IsReg=true,  Reg=number
//   - number     ("123")        → IsNum=true,  Num=int64
//   - symbol ref ("main", "L1") → IsSym=true,  Sym=name
//   - HI20 of a symbol          → IsSym=true,  Sym=name, Mod=ModHI
//   - LO12 of a symbol          → IsSym=true,  Sym=name, Mod=ModLO
//   - PC-relative HI/LO         → IsSym=true,  Mod=ModPCHI or ModPCLO
//
// %hi(sym), %lo(sym), %pcrel_hi(sym), %pcrel_lo(label) are recognized
// modifiers — the same names the GNU assembler uses, so existing source
// works.
type OperandKind int

const (
	OpReg OperandKind = iota
	OpNum
	OpSym
)

type Modifier int

const (
	ModNone   Modifier = iota
	ModHI              // %hi(sym)
	ModLO              // %lo(sym)
	ModPCHI            // %pcrel_hi(sym)
	ModPCLO            // %pcrel_lo(label)
)

type Operand struct {
	Kind OperandKind
	Reg  uint32
	Num  int64
	Sym  string
	Mod  Modifier
}

// StmtKind discriminates the three statement shapes we care about.
type StmtKind int

const (
	StmtLabel StmtKind = iota
	StmtDirective
	StmtInstr
)

// Stmt is one source-line statement.
type Stmt struct {
	Kind StmtKind

	// For labels:
	Label string

	// For directives: directive name without the leading '.'.
	// Operands are stored alongside.
	Directive string

	// For instructions:
	Mnemonic string

	Operands []Operand

	// String literals (.ascii). Stored separately because operands are
	// numeric/symbolic.
	Strings []string

	// Source location for diagnostics.
	File string
	Line int
}
