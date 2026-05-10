package encoder

import (
	"fmt"

	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/isa"
	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/common/reloc"
)

// encodeInstr emits one statement to the .text section. `pc` is the byte
// offset of this instruction within .text — used both for placeholder PC math
// and as the relocation site offset.
//
// Returns the encoded words. The caller is responsible for splatting them at
// .text[pc..pc+4N].
func (e *encoder) encodeInstr(pc uint32, s *parser.Stmt) ([]uint32, error) {
	mn := s.Mnemonic

	// First, check pseudo-instructions. We expand them before falling
	// through to the real opcode table.
	if words, ok, err := e.expandPseudo(pc, s); err != nil {
		return nil, err
	} else if ok {
		return words, nil
	}

	op, ok := isa.LookupOp(mn)
	if !ok {
		return nil, fmt.Errorf("unknown instruction %q", mn)
	}

	switch op.Fmt {
	case isa.FmtR:
		return e.encR(pc, op, s)
	case isa.FmtI:
		return e.encI(pc, op, s)
	case isa.FmtIShift:
		return e.encIShift(pc, op, s)
	case isa.FmtS:
		return e.encS(pc, op, s)
	case isa.FmtB:
		return e.encB(pc, op, s)
	case isa.FmtU:
		return e.encU(pc, op, s)
	case isa.FmtJ:
		return e.encJ(pc, op, s)
	case isa.FmtSys, isa.FmtFence:
		w, err := isa.Assemble(op, isa.Operands{})
		return []uint32{w}, err
	}

	return nil, fmt.Errorf("internal: unhandled format for %s", mn)
}

// ───────── operand helpers ──────────────────────────────────────────────

func mustReg(o parser.Operand) (uint32, error) {
	if o.Kind != parser.OpReg {
		return 0, fmt.Errorf("expected register, got %v", o)
	}
	return o.Reg, nil
}

func mustNum(o parser.Operand) (int64, error) {
	if o.Kind != parser.OpNum {
		return 0, fmt.Errorf("expected number, got %v", o)
	}
	return o.Num, nil
}

func need(s *parser.Stmt, n int) error {
	if len(s.Operands) != n {
		return fmt.Errorf("%s expects %d operands, got %d", s.Mnemonic, n, len(s.Operands))
	}
	return nil
}

// emitReloc appends a relocation entry pointing at the instruction at
// .text[pc].
func (e *encoder) emitReloc(pc uint32, t reloc.Type, symIdx uint32, addend int32) {
	e.m.Relocs = append(e.m.Relocs, obj.Reloc{
		SectionIdx: uint8(e.textIdx),
		Type:       t,
		Offset:     pc,
		SymIdx:     symIdx,
		Addend:     addend,
	})
}

// ───────── format encoders ─────────────────────────────────────────────

func (e *encoder) encR(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 3); err != nil {
		return nil, err
	}
	rd, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}
	rs1, err := mustReg(s.Operands[1])
	if err != nil {
		return nil, err
	}
	rs2, err := mustReg(s.Operands[2])
	if err != nil {
		return nil, err
	}
	w, err := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: rs1, Rs2: rs2})
	return []uint32{w}, err
}

// encI handles the I-type formats. Three sub-cases:
//
//   loads:  lw rd, imm(rs1)        — 3 operands: rd, imm, rs1 (memory form)
//   alu:    addi rd, rs1, imm      — 3 operands
//   jalr:   jalr rd, rs1, imm      — 3 operands  OR  jalr rd, imm(rs1)
//
// For lw/jalr with the memory form, the parser produced operands in
// (rd, imm, rs1) order, which is the same as the alu form. Convenient.
func (e *encoder) encI(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 3); err != nil {
		return nil, err
	}
	rd, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}

	// Load: rd, imm(rs1) → operands [rd, imm, rs1]
	// ALU:  rd, rs1, imm → operands [rd, rs1, imm]
	// jalr: rd, imm(rs1) → [rd, imm, rs1]
	// jalr: rd, rs1, imm → [rd, rs1, imm]
	//
	// Discriminate by whether operand[1] is a register or a number/symbol.
	isLoad := isLoadOpcode(op.Opcode)
	memForm := s.Operands[1].Kind != parser.OpReg

	var rs1 uint32
	var imm parser.Operand
	if memForm {
		imm = s.Operands[1]
		rs1, err = mustReg(s.Operands[2])
		if err != nil {
			return nil, err
		}
	} else {
		rs1, err = mustReg(s.Operands[1])
		if err != nil {
			return nil, err
		}
		imm = s.Operands[2]
	}

	var immVal int32
	switch imm.Kind {
	case parser.OpNum:
		immVal = int32(imm.Num)
	case parser.OpSym:
		// Symbolic operand → emit a relocation. Bytes will be patched.
		symIdx, err := e.lookupSym(imm.Sym, *s)
		if err != nil {
			return nil, err
		}
		switch imm.Mod {
		case parser.ModLO:
			e.emitReloc(pc, reloc.R_RV32_LO12_I, symIdx, 0)
		case parser.ModPCLO:
			// %pcrel_lo(label) — Addend is *unused* in our flavor; the
			// linker matches by symbol value (the label points at the
			// paired auipc).
			e.emitReloc(pc, reloc.R_RV32_PCREL_LO12_I, symIdx, 0)
		case parser.ModNone:
			// A plain symbol in an I-type imm slot is unusual outside of
			// branches/jumps. We accept it as an absolute LO12.
			e.emitReloc(pc, reloc.R_RV32_LO12_I, symIdx, 0)
		default:
			return nil, fmt.Errorf("modifier %v not valid for I-type immediate", imm.Mod)
		}
		// Placeholder zero — linker overwrites.
		immVal = 0
	default:
		return nil, fmt.Errorf("bad I-type operand")
	}
	_ = isLoad

	w, err := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: rs1, Imm: immVal})
	return []uint32{w}, err
}

// encIShift handles slli/srli/srai (rd, rs1, shamt).
func (e *encoder) encIShift(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 3); err != nil {
		return nil, err
	}
	rd, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}
	rs1, err := mustReg(s.Operands[1])
	if err != nil {
		return nil, err
	}
	n, err := mustNum(s.Operands[2])
	if err != nil {
		return nil, err
	}
	w, err := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: rs1, Shamt: uint32(n)})
	return []uint32{w}, err
}

// encS — sw/sh/sb rs2, imm(rs1)  → operands [rs2, imm, rs1].
func (e *encoder) encS(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 3); err != nil {
		return nil, err
	}
	rs2, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}
	rs1, err := mustReg(s.Operands[2])
	if err != nil {
		return nil, err
	}

	imm := s.Operands[1]
	var immVal int32
	switch imm.Kind {
	case parser.OpNum:
		immVal = int32(imm.Num)
	case parser.OpSym:
		symIdx, err := e.lookupSym(imm.Sym, *s)
		if err != nil {
			return nil, err
		}
		switch imm.Mod {
		case parser.ModLO, parser.ModNone:
			e.emitReloc(pc, reloc.R_RV32_LO12_S, symIdx, 0)
		case parser.ModPCLO:
			// Not commonly needed for stores; supported for symmetry.
			return nil, fmt.Errorf("%%pcrel_lo not supported in store form")
		default:
			return nil, fmt.Errorf("modifier %v not valid in store", imm.Mod)
		}
	default:
		return nil, fmt.Errorf("bad S-type operand")
	}

	w, err := isa.Assemble(op, isa.Operands{Rs1: rs1, Rs2: rs2, Imm: immVal})
	return []uint32{w}, err
}

// encB — beq rs1, rs2, target.
func (e *encoder) encB(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 3); err != nil {
		return nil, err
	}
	rs1, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}
	rs2, err := mustReg(s.Operands[1])
	if err != nil {
		return nil, err
	}

	target := s.Operands[2]
	switch target.Kind {
	case parser.OpSym:
		symIdx, err := e.lookupSym(target.Sym, *s)
		if err != nil {
			return nil, err
		}
		e.emitReloc(pc, reloc.R_RV32_BRANCH, symIdx, 0)
		// emit a placeholder branch with offset 0
		w, _ := isa.Assemble(op, isa.Operands{Rs1: rs1, Rs2: rs2, Imm: 0})
		return []uint32{w}, nil
	case parser.OpNum:
		w, err := isa.Assemble(op, isa.Operands{Rs1: rs1, Rs2: rs2, Imm: int32(target.Num)})
		return []uint32{w}, err
	}
	return nil, fmt.Errorf("bad branch target")
}

// encU — lui / auipc, rd, imm.
func (e *encoder) encU(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 2); err != nil {
		return nil, err
	}
	rd, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}

	imm := s.Operands[1]
	switch imm.Kind {
	case parser.OpNum:
		// User typed `lui x1, 0x12345` — bit pattern is the low-20-bits
		// of imm shifted into [31:12]. We accept it and let isa.Assemble
		// mask. If user wrote a full 32-bit value, that also works as
		// long as the low 12 bits are zero — but to match GNU as we
		// shift the 20-bit constant ourselves.
		var u uint32
		if imm.Num >= 0 && imm.Num < (1<<20) {
			u = uint32(imm.Num) << 12
		} else {
			u = uint32(imm.Num) // already shifted (or signed)
		}
		w, err := isa.Assemble(op, isa.Operands{Rd: rd, UImm: u})
		return []uint32{w}, err
	case parser.OpSym:
		symIdx, err := e.lookupSym(imm.Sym, *s)
		if err != nil {
			return nil, err
		}
		switch imm.Mod {
		case parser.ModHI, parser.ModNone:
			if op.Name == "auipc" {
				e.emitReloc(pc, reloc.R_RV32_PCREL_HI20, symIdx, 0)
			} else {
				e.emitReloc(pc, reloc.R_RV32_HI20, symIdx, 0)
			}
		case parser.ModPCHI:
			e.emitReloc(pc, reloc.R_RV32_PCREL_HI20, symIdx, 0)
		default:
			return nil, fmt.Errorf("modifier %v not valid for U-type", imm.Mod)
		}
		// Placeholder
		w, _ := isa.Assemble(op, isa.Operands{Rd: rd, UImm: 0})
		return []uint32{w}, nil
	}
	return nil, fmt.Errorf("bad U-type operand")
}

// encJ — jal rd, target.
func (e *encoder) encJ(pc uint32, op isa.OpDef, s *parser.Stmt) ([]uint32, error) {
	if err := need(s, 2); err != nil {
		return nil, err
	}
	rd, err := mustReg(s.Operands[0])
	if err != nil {
		return nil, err
	}
	target := s.Operands[1]
	switch target.Kind {
	case parser.OpSym:
		symIdx, err := e.lookupSym(target.Sym, *s)
		if err != nil {
			return nil, err
		}
		e.emitReloc(pc, reloc.R_RV32_JAL, symIdx, 0)
		w, _ := isa.Assemble(op, isa.Operands{Rd: rd, Imm: 0})
		return []uint32{w}, nil
	case parser.OpNum:
		w, err := isa.Assemble(op, isa.Operands{Rd: rd, Imm: int32(target.Num)})
		return []uint32{w}, err
	}
	return nil, fmt.Errorf("bad jal target")
}

func isLoadOpcode(op uint32) bool { return op == 0x03 }
