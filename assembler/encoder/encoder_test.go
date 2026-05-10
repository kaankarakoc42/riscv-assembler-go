package encoder

import (
	"encoding/binary"
	"testing"

	"github.com/edu/rvtoolchain/assembler/lexer"
	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/common/reloc"
)

func assemble(t *testing.T, src string) *obj.Module {
	t.Helper()
	toks, err := lexer.Lex("t.s", []byte(src))
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	stmts, err := parser.Parse("t.s", toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, err := Encode(stmts)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return m
}

func TestEncodeSimpleAddi(t *testing.T) {
	m := assemble(t, `
.text
main:
    addi x1, x0, 5
`)
	textIdx := m.FindSection(".text")
	if textIdx < 0 {
		t.Fatal("no .text section")
	}
	w := binary.LittleEndian.Uint32(m.Sections[textIdx].Data)
	if w != 0x00500093 {
		t.Errorf("addi x1,x0,5 = 0x%08X want 0x00500093", w)
	}
}

func TestForwardBranchEmitsRelocation(t *testing.T) {
	m := assemble(t, `
.text
main:
    beq x1, x2, target
    addi x0, x0, 0
target:
    addi x0, x0, 0
`)
	if len(m.Relocs) != 1 {
		t.Fatalf("expected 1 reloc, got %d", len(m.Relocs))
	}
	r := m.Relocs[0]
	if r.Type != reloc.R_RV32_BRANCH {
		t.Errorf("reloc type = %v want R_RV32_BRANCH", r.Type)
	}
	if r.Offset != 0 {
		t.Errorf("reloc offset = %d want 0", r.Offset)
	}
	sym := m.Symbols[r.SymIdx]
	if sym.Name != "target" {
		t.Errorf("reloc points at %q, want \"target\"", sym.Name)
	}
	if sym.Value != 8 {
		t.Errorf("target offset = %d want 8", sym.Value)
	}
}

func TestGlobalDirective(t *testing.T) {
	m := assemble(t, `
.global main
.text
main:
    addi x0, x0, 0
`)
	for _, s := range m.Symbols {
		if s.Name == "main" {
			if s.Bind != obj.BindGlobal {
				t.Errorf("main bind = %v, want GLOBAL", s.Bind)
			}
			return
		}
	}
	t.Fatal("main symbol missing")
}

func TestExternSymbolGetsExternBinding(t *testing.T) {
	m := assemble(t, `
.text
main:
    call printf
    ret
`)
	var found bool
	for _, s := range m.Symbols {
		if s.Name == "printf" {
			found = true
			if s.Bind != obj.BindExtern {
				t.Errorf("printf bind = %v want EXTERN", s.Bind)
			}
		}
	}
	if !found {
		t.Fatal("printf symbol missing")
	}
}

func TestDataWordWithRelocation(t *testing.T) {
	m := assemble(t, `
.data
ptr:
    .word target
.text
target:
    addi x0, x0, 0
`)
	dataIdx := m.FindSection(".data")
	if dataIdx < 0 {
		t.Fatal("no .data section")
	}
	if len(m.Relocs) != 1 || m.Relocs[0].Type != reloc.R_RV32_32 {
		t.Fatalf("expected one R_RV32_32 reloc, got %+v", m.Relocs)
	}
	if m.Relocs[0].SectionIdx != uint8(dataIdx) {
		t.Errorf("reloc section = %d want %d", m.Relocs[0].SectionIdx, dataIdx)
	}
}

func TestSplitHILOPositive(t *testing.T) {
	hi, lo := splitHILO(0x12345678)
	// lo = 0x678, hi = 0x12345 (no carry since bit 11 of lo is 0)
	if hi != 0x12345000 || lo != 0x678 {
		t.Errorf("splitHILO(0x12345678) = (0x%X, 0x%X), want (0x12345000, 0x678)", hi, lo)
	}
}

func TestSplitHILOWithCarry(t *testing.T) {
	// lo of 0xFFF needs the carry: addi rd, rd, -1
	hi, lo := splitHILO(0x12345FFF)
	// lo=0xFFF, sign-extended = -1, so hi must be 0x12346 (rounded up)
	if hi != 0x12346000 || lo != 0xFFF {
		t.Errorf("splitHILO(0x12345FFF) = (0x%X, 0x%X), want (0x12346000, 0xFFF)", hi, lo)
	}
}

func TestPseudoNop(t *testing.T) {
	m := assemble(t, ".text\nnop\n")
	w := binary.LittleEndian.Uint32(m.Sections[m.FindSection(".text")].Data)
	if w != 0x00000013 {
		t.Errorf("nop = 0x%08X want 0x00000013", w)
	}
}

func TestPseudoMv(t *testing.T) {
	m := assemble(t, ".text\nmv x5, x6\n")
	// mv x5, x6 = addi x5, x6, 0 → 0x00030293
	w := binary.LittleEndian.Uint32(m.Sections[m.FindSection(".text")].Data)
	if w != 0x00030293 {
		t.Errorf("mv = 0x%08X want 0x00030293", w)
	}
}

func TestPseudoRet(t *testing.T) {
	m := assemble(t, ".text\nret\n")
	// ret = jalr x0, 0(x1) = 0x00008067
	w := binary.LittleEndian.Uint32(m.Sections[m.FindSection(".text")].Data)
	if w != 0x00008067 {
		t.Errorf("ret = 0x%08X want 0x00008067", w)
	}
}

func TestDuplicateLabelError(t *testing.T) {
	src := ".text\nfoo:\nfoo:\n"
	toks, _ := lexer.Lex("t.s", []byte(src))
	stmts, _ := parser.Parse("t.s", toks)
	if _, err := Encode(stmts); err == nil {
		t.Fatal("expected duplicate label error")
	}
}
