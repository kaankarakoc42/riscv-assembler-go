package parser

import (
	"testing"

	"github.com/edu/rvtoolchain/assembler/lexer"
)

func parse(t *testing.T, src string) []Stmt {
	t.Helper()
	toks, err := lexer.Lex("t.s", []byte(src))
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	stmts, err := Parse("t.s", toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return stmts
}

func TestLabelAndInstruction(t *testing.T) {
	stmts := parse(t, "main:\n  addi x1, x0, 5\n")
	if len(stmts) != 2 {
		t.Fatalf("got %d stmts: %+v", len(stmts), stmts)
	}
	if stmts[0].Kind != StmtLabel || stmts[0].Label != "main" {
		t.Errorf("expected label main, got %+v", stmts[0])
	}
	if stmts[1].Kind != StmtInstr || stmts[1].Mnemonic != "addi" {
		t.Errorf("expected addi, got %+v", stmts[1])
	}
	if got := stmts[1].Operands; len(got) != 3 {
		t.Fatalf("addi expected 3 operands, got %d (%+v)", len(got), got)
	}
}

func TestLoadStoreMemoryOperand(t *testing.T) {
	stmts := parse(t, "lw x1, 8(x2)\n")
	if len(stmts) != 1 {
		t.Fatalf("got %d stmts", len(stmts))
	}
	ops := stmts[0].Operands
	if len(ops) != 3 {
		t.Fatalf("expected 3 operands (rd, imm, base), got %d: %+v", len(ops), ops)
	}
	if ops[0].Kind != OpReg || ops[0].Reg != 1 {
		t.Errorf("op0 not x1: %+v", ops[0])
	}
	if ops[1].Kind != OpNum || ops[1].Num != 8 {
		t.Errorf("op1 not num 8: %+v", ops[1])
	}
	if ops[2].Kind != OpReg || ops[2].Reg != 2 {
		t.Errorf("op2 not x2: %+v", ops[2])
	}
}

func TestSymbolBranch(t *testing.T) {
	stmts := parse(t, "beq x1, x2, target\n")
	ops := stmts[0].Operands
	if ops[2].Kind != OpSym || ops[2].Sym != "target" {
		t.Errorf("expected symbol operand, got %+v", ops[2])
	}
}

func TestHiLoModifiers(t *testing.T) {
	stmts := parse(t, "lui x1, hi(msg)\naddi x1, x1, lo(msg)\n")
	if stmts[0].Operands[1].Mod != ModHI {
		t.Errorf("expected ModHI, got %v", stmts[0].Operands[1].Mod)
	}
	if stmts[1].Operands[2].Mod != ModLO {
		t.Errorf("expected ModLO, got %v", stmts[1].Operands[2].Mod)
	}
}

func TestDirectives(t *testing.T) {
	stmts := parse(t, ".global main\n.word 0xdeadbeef\n.ascii \"hi\"\n")
	if stmts[0].Kind != StmtDirective || stmts[0].Directive != "global" {
		t.Errorf("expected .global, got %+v", stmts[0])
	}
	if stmts[1].Kind != StmtDirective || stmts[1].Directive != "word" {
		t.Errorf("expected .word, got %+v", stmts[1])
	}
	if stmts[2].Strings[0] != "hi" {
		t.Errorf("expected ascii 'hi', got %+v", stmts[2])
	}
}

func TestLabelOnSameLineAsInstr(t *testing.T) {
	stmts := parse(t, "loop: addi x1, x1, 1\n")
	if len(stmts) != 2 || stmts[0].Kind != StmtLabel || stmts[1].Kind != StmtInstr {
		t.Fatalf("expected [label, instr], got %+v", stmts)
	}
}
