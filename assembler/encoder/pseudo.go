package encoder

import (
	"fmt"

	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/isa"
	"github.com/edu/rvtoolchain/common/reloc"
)

// expandPseudo handles the convenience aliases that don't appear directly in
// the RV32I instruction set but are universally accepted by RISC-V tooling.
//
// Returns (words, true, nil) if mn was a pseudo and was expanded; (nil, false,
// nil) if not. Returns (nil, false, err) on a bad use.
func (e *encoder) expandPseudo(pc uint32, s *parser.Stmt) ([]uint32, bool, error) {
	switch s.Mnemonic {

	case "nop":
		// nop = addi x0, x0, 0
		op, _ := isa.LookupOp("addi")
		w, _ := isa.Assemble(op, isa.Operands{})
		return []uint32{w}, true, nil

	case "mv":
		// mv rd, rs  →  addi rd, rs, 0
		if err := need(s, 2); err != nil {
			return nil, true, err
		}
		rd, _ := mustReg(s.Operands[0])
		rs, _ := mustReg(s.Operands[1])
		op, _ := isa.LookupOp("addi")
		w, _ := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: rs, Imm: 0})
		return []uint32{w}, true, nil

	case "neg":
		// neg rd, rs  →  sub rd, x0, rs
		if err := need(s, 2); err != nil {
			return nil, true, err
		}
		rd, _ := mustReg(s.Operands[0])
		rs, _ := mustReg(s.Operands[1])
		op, _ := isa.LookupOp("sub")
		w, _ := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: 0, Rs2: rs})
		return []uint32{w}, true, nil

	case "not":
		// not rd, rs  →  xori rd, rs, -1
		if err := need(s, 2); err != nil {
			return nil, true, err
		}
		rd, _ := mustReg(s.Operands[0])
		rs, _ := mustReg(s.Operands[1])
		op, _ := isa.LookupOp("xori")
		w, _ := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: rs, Imm: -1})
		return []uint32{w}, true, nil

	case "ret":
		// ret  →  jalr x0, 0(x1)
		op, _ := isa.LookupOp("jalr")
		w, _ := isa.Assemble(op, isa.Operands{Rd: 0, Rs1: 1, Imm: 0})
		return []uint32{w}, true, nil

	case "j":
		// j label  →  jal x0, label
		if err := need(s, 1); err != nil {
			return nil, true, err
		}
		op, _ := isa.LookupOp("jal")
		t := s.Operands[0]
		switch t.Kind {
		case parser.OpSym:
			symIdx, err := e.lookupSym(t.Sym, *s)
			if err != nil {
				return nil, true, err
			}
			e.emitReloc(pc, reloc.R_RV32_JAL, symIdx, 0)
			w, _ := isa.Assemble(op, isa.Operands{Rd: 0, Imm: 0})
			return []uint32{w}, true, nil
		case parser.OpNum:
			w, err := isa.Assemble(op, isa.Operands{Rd: 0, Imm: int32(t.Num)})
			return []uint32{w}, true, err
		}
		return nil, true, fmt.Errorf("bad operand to j")

	case "jr":
		// jr rs  →  jalr x0, 0(rs)
		if err := need(s, 1); err != nil {
			return nil, true, err
		}
		rs, _ := mustReg(s.Operands[0])
		op, _ := isa.LookupOp("jalr")
		w, _ := isa.Assemble(op, isa.Operands{Rd: 0, Rs1: rs, Imm: 0})
		return []uint32{w}, true, nil

	// ─── conditional branches against zero ───────────────────────────────
	case "beqz", "bnez", "bltz", "bgez", "blez", "bgtz":
		if err := need(s, 2); err != nil {
			return nil, true, err
		}
		rs, _ := mustReg(s.Operands[0])
		// Map to the canonical 3-operand branch.
		var realMn string
		var rs1, rs2 uint32
		switch s.Mnemonic {
		case "beqz":
			realMn, rs1, rs2 = "beq", rs, 0
		case "bnez":
			realMn, rs1, rs2 = "bne", rs, 0
		case "bltz":
			realMn, rs1, rs2 = "blt", rs, 0
		case "bgez":
			realMn, rs1, rs2 = "bge", rs, 0
		case "blez":
			realMn, rs1, rs2 = "bge", 0, rs
		case "bgtz":
			realMn, rs1, rs2 = "blt", 0, rs
		}
		op, _ := isa.LookupOp(realMn)
		t := s.Operands[1]
		switch t.Kind {
		case parser.OpSym:
			symIdx, err := e.lookupSym(t.Sym, *s)
			if err != nil {
				return nil, true, err
			}
			e.emitReloc(pc, reloc.R_RV32_BRANCH, symIdx, 0)
			w, _ := isa.Assemble(op, isa.Operands{Rs1: rs1, Rs2: rs2})
			return []uint32{w}, true, nil
		case parser.OpNum:
			w, err := isa.Assemble(op, isa.Operands{Rs1: rs1, Rs2: rs2, Imm: int32(t.Num)})
			return []uint32{w}, true, err
		}
		return nil, true, fmt.Errorf("bad branch target")

	// ─── li rd, imm  (the simple 2-instruction expansion) ────────────────
	//
	// Implementation: split imm into HI20 + LO12 such that
	//   ((hi << 12) + sign_extend(lo)) == imm
	// Because the lo12 is sign-extended, we add 1 to hi when bit 11 of
	// lo12 is set.
	case "li":
		if err := need(s, 2); err != nil {
			return nil, true, err
		}
		rd, _ := mustReg(s.Operands[0])
		n, err := mustNum(s.Operands[1])
		if err != nil {
			return nil, true, err
		}
		imm := int32(n)
		// short form: fits in 12-bit signed → single addi
		if imm >= -2048 && imm <= 2047 {
			op, _ := isa.LookupOp("addi")
			w, _ := isa.Assemble(op, isa.Operands{Rd: rd, Rs1: 0, Imm: imm})
			// To keep section size predictable (pass1 said 2 words for li),
			// we still emit 2 instructions: addi + nop. Educational tradeoff.
			nop, _ := isa.Assemble(op, isa.Operands{})
			return []uint32{w, nop}, true, nil
		}
		hi, lo := splitHILO(uint32(imm))
		lui, _ := isa.LookupOp("lui")
		addi, _ := isa.LookupOp("addi")
		w1, _ := isa.Assemble(lui, isa.Operands{Rd: rd, UImm: hi})
		w2, err := isa.Assemble(addi, isa.Operands{Rd: rd, Rs1: rd, Imm: int32(int32(lo<<20) >> 20)})
		if err != nil {
			return nil, true, err
		}
		return []uint32{w1, w2}, true, nil

	// ─── la rd, sym ──────────────────────────────────────────────────────
	//   auipc rd, %pcrel_hi(sym)
	//   addi  rd, rd, %pcrel_lo(.label)
	//
	// where ".label" is the address of the auipc itself. Because we don't
	// emit a real label, our flavor of R_RV32_PCREL_LO12_I uses the *symbol*
	// directly with the linker rediscovering the paired auipc. Simpler.
	case "la":
		if err := need(s, 2); err != nil {
			return nil, true, err
		}
		rd, _ := mustReg(s.Operands[0])
		t := s.Operands[1]
		if t.Kind != parser.OpSym {
			return nil, true, fmt.Errorf("la expects a symbol")
		}
		symIdx, err := e.lookupSym(t.Sym, *s)
		if err != nil {
			return nil, true, err
		}
		auipc, _ := isa.LookupOp("auipc")
		addi, _ := isa.LookupOp("addi")
		// auipc at pc; addi at pc+4
		e.emitReloc(pc, reloc.R_RV32_PCREL_HI20, symIdx, 0)
		e.emitReloc(pc+4, reloc.R_RV32_PCREL_LO12_I, symIdx, 0)
		w1, _ := isa.Assemble(auipc, isa.Operands{Rd: rd, UImm: 0})
		w2, _ := isa.Assemble(addi, isa.Operands{Rd: rd, Rs1: rd, Imm: 0})
		return []uint32{w1, w2}, true, nil

	// ─── call sym ────────────────────────────────────────────────────────
	//   auipc x1, %pcrel_hi(sym)
	//   jalr  x1, x1, %pcrel_lo(sym)
	case "call":
		if err := need(s, 1); err != nil {
			return nil, true, err
		}
		t := s.Operands[0]
		if t.Kind != parser.OpSym {
			return nil, true, fmt.Errorf("call expects a symbol")
		}
		symIdx, err := e.lookupSym(t.Sym, *s)
		if err != nil {
			return nil, true, err
		}
		auipc, _ := isa.LookupOp("auipc")
		jalr, _ := isa.LookupOp("jalr")
		e.emitReloc(pc, reloc.R_RV32_PCREL_HI20, symIdx, 0)
		e.emitReloc(pc+4, reloc.R_RV32_PCREL_LO12_I, symIdx, 0)
		w1, _ := isa.Assemble(auipc, isa.Operands{Rd: 1, UImm: 0})
		w2, _ := isa.Assemble(jalr, isa.Operands{Rd: 1, Rs1: 1, Imm: 0})
		return []uint32{w1, w2}, true, nil

	// ─── tail sym ────────────────────────────────────────────────────────
	//   auipc x6, %pcrel_hi(sym)
	//   jalr  x0, x6, %pcrel_lo(sym)
	case "tail":
		if err := need(s, 1); err != nil {
			return nil, true, err
		}
		t := s.Operands[0]
		if t.Kind != parser.OpSym {
			return nil, true, fmt.Errorf("tail expects a symbol")
		}
		symIdx, err := e.lookupSym(t.Sym, *s)
		if err != nil {
			return nil, true, err
		}
		auipc, _ := isa.LookupOp("auipc")
		jalr, _ := isa.LookupOp("jalr")
		e.emitReloc(pc, reloc.R_RV32_PCREL_HI20, symIdx, 0)
		e.emitReloc(pc+4, reloc.R_RV32_PCREL_LO12_I, symIdx, 0)
		w1, _ := isa.Assemble(auipc, isa.Operands{Rd: 6, UImm: 0})
		w2, _ := isa.Assemble(jalr, isa.Operands{Rd: 0, Rs1: 6, Imm: 0})
		return []uint32{w1, w2}, true, nil
	}

	return nil, false, nil
}

// splitHILO splits a 32-bit absolute value into the (hi20<<12, lo12) pair
// such that
//
//   final = (hi20 << 12) + sign_extend12(lo12)
//
// Because lo12 is sign-extended when consumed by addi, if its bit 11 is set
// we must increment hi20 by one to compensate. This is the standard RISC-V
// HI20/LO12 split.
//
// Returns (hi20<<12, lo12_in_low12_bits).
func splitHILO(v uint32) (hi, lo uint32) {
	lo = v & 0xFFF
	hi = v >> 12
	if lo&0x800 != 0 { // bit 11 set → lo12 will sign-extend negative
		hi++
	}
	return hi << 12, lo
}
