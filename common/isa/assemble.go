package isa

import "fmt"

// Operands carries everything an instruction encoder might need. Not all
// fields are used by every format; the encoder picks what it needs based on
// OpDef.Fmt.
type Operands struct {
	Rd, Rs1, Rs2 uint32
	Imm          int32  // already sign-extended where applicable
	UImm         uint32 // raw 32-bit immediate for U-type (top 20 bits used)
	Shamt        uint32 // 0..31 shift amount for FmtIShift
}

// Assemble packs the given OpDef + operands into a 32-bit word.
//
// This is the *pure* path: imm/shamt must already be in range. The caller
// (parser/encoder or relocation engine) is responsible for validation. Out
// of range bits are silently masked by the underlying EncodeX helpers.
func Assemble(op OpDef, o Operands) (uint32, error) {
	switch op.Fmt {
	case FmtR:
		return EncodeR(op.Opcode, o.Rd, op.Funct3, o.Rs1, o.Rs2, op.Funct7), nil

	case FmtI:
		// Bounds check for sanity: 12-bit signed.
		if o.Imm < -2048 || o.Imm > 2047 {
			return 0, fmt.Errorf("isa: immediate %d out of range for %s (must fit in 12-bit signed)", o.Imm, op.Name)
		}
		return EncodeI(op.Opcode, o.Rd, op.Funct3, o.Rs1, o.Imm), nil

	case FmtIShift:
		if o.Shamt > 31 {
			return 0, fmt.Errorf("isa: shamt %d out of range for %s (0..31)", o.Shamt, op.Name)
		}
		// shamt occupies imm[4:0], funct7 occupies imm[11:5]
		imm := int32((op.Funct7 << 5) | (o.Shamt & 0x1F))
		return EncodeI(op.Opcode, o.Rd, op.Funct3, o.Rs1, imm), nil

	case FmtS:
		if o.Imm < -2048 || o.Imm > 2047 {
			return 0, fmt.Errorf("isa: store offset %d out of range for %s", o.Imm, op.Name)
		}
		return EncodeS(op.Opcode, op.Funct3, o.Rs1, o.Rs2, o.Imm), nil

	case FmtB:
		if o.Imm&1 != 0 {
			return 0, fmt.Errorf("isa: branch target offset %d is not 2-byte aligned", o.Imm)
		}
		if o.Imm < -4096 || o.Imm > 4094 {
			return 0, fmt.Errorf("isa: branch offset %d out of range for %s (±4 KiB)", o.Imm, op.Name)
		}
		return EncodeB(op.Opcode, op.Funct3, o.Rs1, o.Rs2, o.Imm), nil

	case FmtU:
		// Upper 20-bit immediate. We accept the *raw 32-bit value* and only
		// retain the high 20 bits — this matches how programmers write
		// `lui x1, 0x12345` (== 0x12345000 in the produced register).
		return EncodeU(op.Opcode, o.Rd, o.UImm), nil

	case FmtJ:
		if o.Imm&1 != 0 {
			return 0, fmt.Errorf("isa: jal target offset %d is not 2-byte aligned", o.Imm)
		}
		if o.Imm < -(1<<20) || o.Imm > (1<<20)-2 {
			return 0, fmt.Errorf("isa: jal offset %d out of range (±1 MiB)", o.Imm)
		}
		return EncodeJ(op.Opcode, o.Rd, o.Imm), nil

	case FmtSys:
		// ecall = 0x00000073, ebreak = 0x00100073
		switch op.Name {
		case "ecall":
			return 0x00000073, nil
		case "ebreak":
			return 0x00100073, nil
		}
		return 0, fmt.Errorf("isa: unknown system instruction %s", op.Name)

	case FmtFence:
		// fence iorw,iorw — pred=rs1=0xF, succ=rd=0xF, fm=0
		return 0x0FF0000F, nil
	}
	return 0, fmt.Errorf("isa: unsupported format for %s", op.Name)
}
