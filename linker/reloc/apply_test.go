package reloc

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/edu/rvtoolchain/assembler/encoder"
	"github.com/edu/rvtoolchain/assembler/lexer"
	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/linker"
	"github.com/edu/rvtoolchain/linker/script"
)

// asmModule turns asm source into a Module via the in-process assembler.
// We round-trip through obj.Write/obj.Read so the test exercises the same
// path the real CLI does.
func asmModule(t *testing.T, src string) *obj.Module {
	t.Helper()
	toks, err := lexer.Lex("t.s", []byte(src))
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	stmts, err := parser.Parse("t.s", toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, err := encoder.Encode(stmts)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var buf bytes.Buffer
	if err := obj.Write(&buf, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := obj.Read(buf.Bytes())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return out
}

func linkSingle(t *testing.T, src string) *linker.Image {
	t.Helper()
	m := asmModule(t, src)
	img, err := linker.Link([]linker.Input{{Path: "test.ro", Module: m}}, script.Default())
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := Apply(img); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return img
}

func wordAt(t *testing.T, img *linker.Image, addr uint32) uint32 {
	t.Helper()
	if addr < img.TextBase || addr+4 > img.TextBase+uint32(len(img.TextData)) {
		t.Fatalf("addr 0x%X outside text", addr)
	}
	return binary.LittleEndian.Uint32(img.TextData[addr-img.TextBase:])
}

// ─────────────────────────────────────────────────────────────────────────
// Branch (R_RV32_BRANCH)
// ─────────────────────────────────────────────────────────────────────────

func TestBranchForward(t *testing.T) {
	src := `
.text
main:
    beq x1, x2, target  # PC=0
    addi x0, x0, 0      # PC=4
    addi x0, x0, 0      # PC=8
target:
    addi x0, x0, 0      # PC=12
`
	img := linkSingle(t, src)
	w := wordAt(t, img, 0)
	// beq x1, x2, +12 : encoded with offset 12.
	// imm[12]=0 imm[11]=0 imm[10:5]=0 imm[4:1]=6
	// 0 000000 00010 00001 000 0110 0 1100011 = 0x00208663
	want := uint32(0x00208663)
	if w != want {
		t.Errorf("beq forward = 0x%08X want 0x%08X", w, want)
	}
}

func TestBranchBackward(t *testing.T) {
	src := `
.text
loop:
    addi x1, x1, 1    # PC=0
    bne  x1, x2, loop # PC=4 → -4
`
	img := linkSingle(t, src)
	w := wordAt(t, img, 4)
	// bne x1, x2, -4: imm=-4
	// imm[12]=1, imm[11]=1, imm[10:5]=0x3F, imm[4:1]=0xE
	// = 0xFE209EE3 (rs1=1, rs2=2)
	want := uint32(0xFE209EE3)
	if w != want {
		t.Errorf("bne backward = 0x%08X want 0x%08X", w, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// JAL (R_RV32_JAL)
// ─────────────────────────────────────────────────────────────────────────

func TestJalForward(t *testing.T) {
	src := `
.text
start:
    jal x1, target    # PC=0 → +12
    addi x0, x0, 0    # PC=4
    addi x0, x0, 0    # PC=8
target:
    addi x0, x0, 0    # PC=12
`
	img := linkSingle(t, src)
	w := wordAt(t, img, 0)
	// jal x1, +12: imm[20]=0 imm[19:12]=0 imm[11]=0 imm[10:1]=6
	// = 0x00C000EF
	want := uint32(0x00C000EF)
	if w != want {
		t.Errorf("jal forward = 0x%08X want 0x%08X", w, want)
	}
}

func TestJPseudoForward(t *testing.T) {
	src := `
.text
start:
    j target          # PC=0 → +8 (jal x0)
    addi x0, x0, 0    # PC=4
target:
    addi x0, x0, 0    # PC=8
`
	img := linkSingle(t, src)
	w := wordAt(t, img, 0)
	// j +8 = jal x0, +8: 0x0080006F
	want := uint32(0x0080006F)
	if w != want {
		t.Errorf("j forward = 0x%08X want 0x%08X", w, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// .word  (R_RV32_32)
// ─────────────────────────────────────────────────────────────────────────

func TestWordRelocation(t *testing.T) {
	src := `
.data
ptr:
    .word target
.text
target:
    addi x0, x0, 0
`
	img := linkSingle(t, src)
	// .data lives right after .text in default layout. target is at
	// img.TextBase = 0. ptr's word should hold target's address = 0.
	w := binary.LittleEndian.Uint32(img.DataData[0:4])
	if w != 0 {
		t.Errorf("ptr = 0x%08X, want 0", w)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// la pseudo (PCREL_HI20 + PCREL_LO12_I)
// ─────────────────────────────────────────────────────────────────────────

func TestLaPseudoComputesAddress(t *testing.T) {
	src := `
.text
main:
    la   x5, msg       # PC=0 → auipc; PC=4 → addi
    ret                # PC=8
.data
msg:
    .ascii "hello\n"
`
	img := linkSingle(t, src)

	// After link, msg is at DataBase. Default layout:
	//  TextBase=0, len(text)=12 (3 instr × 4), DataBase=12.
	// la sequence at PC=0 (auipc) and PC=4 (addi):
	//   target = msg = 0x0C
	//   PC of auipc = 0
	//   delta = msg - PC = 12
	//   HI20 = 0x00000  (since 12 < 0x800, no carry)
	//   LO12 = 0x00C
	// So auipc x5, 0  →  0x00000297
	// And  addi  x5, x5, 12 → 0x00C28293
	wAuipc := wordAt(t, img, 0)
	wAddi := wordAt(t, img, 4)
	if wAuipc != 0x00000297 {
		t.Errorf("auipc = 0x%08X want 0x00000297", wAuipc)
	}
	if wAddi != 0x00C28293 {
		t.Errorf("addi  = 0x%08X want 0x00C28293", wAddi)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Multi-file linking with cross-module references
// ─────────────────────────────────────────────────────────────────────────

func TestMultiFileCallResolves(t *testing.T) {
	srcA := `
.global main
.text
main:
    call worker
    ret
`
	srcB := `
.global worker
.text
worker:
    addi x10, x0, 42
    ret
`
	mA := asmModule(t, srcA)
	mB := asmModule(t, srcB)
	img, err := linker.Link([]linker.Input{
		{Path: "a.ro", Module: mA},
		{Path: "b.ro", Module: mB},
	}, script.Default())
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := Apply(img); err != nil {
		t.Fatalf("apply: %v", err)
	}

	g, ok := img.Globals["worker"]
	if !ok {
		t.Fatal("worker not in globals")
	}

	// The call sequence is at addresses 0 and 4 of mA. After linking, mA's
	// .text starts at 0. So:
	//   PC of auipc = 0
	//   delta = worker_addr - 0 = worker_addr
	// Since main is 3 instructions (call=2, ret=1) → 12 bytes, worker
	// starts at 12.
	if g.Address != 12 {
		t.Errorf("worker address = 0x%X want 0x0C", g.Address)
	}

	// Validate the auipc and jalr immediates encode +12.
	wAuipc := binary.LittleEndian.Uint32(img.TextData[0:4])
	wJalr := binary.LittleEndian.Uint32(img.TextData[4:8])
	// auipc x1, 0  = 0x00000097
	if wAuipc != 0x00000097 {
		t.Errorf("auipc = 0x%08X want 0x00000097", wAuipc)
	}
	// jalr x1, x1, +12 = 0x00C080E7
	if wJalr != 0x00C080E7 {
		t.Errorf("jalr  = 0x%08X want 0x00C080E7", wJalr)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Error edges
// ─────────────────────────────────────────────────────────────────────────

func TestUndefinedSymbol(t *testing.T) {
	src := `
.text
main:
    call missing
    ret
`
	m := asmModule(t, src)
	_, err := linker.Link([]linker.Input{{Path: "t.ro", Module: m}}, script.Default())
	if err == nil {
		t.Fatal("expected undefined-symbol error")
	}
}

func TestDuplicateGlobal(t *testing.T) {
	src := `
.global foo
.text
foo:
    addi x0, x0, 0
`
	mA := asmModule(t, src)
	mB := asmModule(t, src)
	_, err := linker.Link([]linker.Input{
		{Path: "a.ro", Module: mA},
		{Path: "b.ro", Module: mB},
	}, script.Default())
	if err == nil {
		t.Fatal("expected duplicate-global error")
	}
}
