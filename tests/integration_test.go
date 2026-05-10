// Package tests holds end-to-end integration tests that drive the same code
// path as the rvasm + rvld CLIs.
package tests

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/edu/rvtoolchain/assembler/encoder"
	"github.com/edu/rvtoolchain/assembler/lexer"
	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/linker"
	"github.com/edu/rvtoolchain/linker/image"
	rl "github.com/edu/rvtoolchain/linker/reloc"
	"github.com/edu/rvtoolchain/linker/script"
)

// asmFile drives the full assembler pipeline for one .s string and returns
// the round-tripped (write+read) Module.
func asmFile(t *testing.T, name, src string) *obj.Module {
	t.Helper()
	toks, err := lexer.Lex(name, []byte(src))
	if err != nil {
		t.Fatalf("%s: lex: %v", name, err)
	}
	stmts, err := parser.Parse(name, toks)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	m, err := encoder.Encode(stmts)
	if err != nil {
		t.Fatalf("%s: encode: %v", name, err)
	}
	var buf bytes.Buffer
	if err := obj.Write(&buf, m); err != nil {
		t.Fatalf("%s: obj.Write: %v", name, err)
	}
	out, err := obj.Read(buf.Bytes())
	if err != nil {
		t.Fatalf("%s: obj.Read: %v", name, err)
	}
	return out
}

// fullLink runs the full pipeline on a slice of (name, src) pairs and
// returns the linked image after relocation.
func fullLink(t *testing.T, sc *script.Script, srcs ...struct{ name, src string }) *linker.Image {
	t.Helper()
	if sc == nil {
		sc = script.Default()
	}
	inputs := make([]linker.Input, 0, len(srcs))
	for _, s := range srcs {
		inputs = append(inputs, linker.Input{Path: s.name, Module: asmFile(t, s.name, s.src)})
	}
	img, err := linker.Link(inputs, sc)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := rl.Apply(img); err != nil {
		t.Fatalf("relocate: %v", err)
	}
	return img
}

// ─── End-to-end "blink-style" demo ────────────────────────────────────────

func TestEndToEnd_Blink(t *testing.T) {
	start := `
.global _start
.global delay
.extern main
.text
_start:
    li   sp, 0x2000
    call main
hang:
    j    hang
delay:
    mv   t0, a0
delay_loop:
    addi t0, t0, -1
    bnez t0, delay_loop
    ret
`
	blink := `
.global main
.extern delay
.text
main:
    addi sp, sp, -16
    sw   ra, 12(sp)
    li   t1, 0x80000000
    li   t2, 0x01
loop:
    sw   t2, 0(t1)
    li   a0, 0x40
    call delay
    slli t2, t2, 1
    li   t3, 0x40
    bne  t2, t3, no_wrap
    li   t2, 0x01
no_wrap:
    j    loop
`
	img := fullLink(t, nil,
		struct{ name, src string }{"start.s", start},
		struct{ name, src string }{"blink.s", blink},
	)

	// _start at 0; main resolves and is reachable via the pcrel-call.
	if img.Globals["_start"].Address != 0 {
		t.Errorf("_start should be at 0, got 0x%X", img.Globals["_start"].Address)
	}
	if g, ok := img.Globals["main"]; !ok || g.Address == 0 {
		t.Errorf("main not linked (%+v)", g)
	}

	// First word should be the LUI of `li sp, 0x2000` → 0x00002137.
	first := binary.LittleEndian.Uint32(img.TextData[0:4])
	if first != 0x00002137 {
		t.Errorf("first word = 0x%08X want 0x00002137", first)
	}

	// $readmemh emit should produce hex words, one per line.
	var memBuf bytes.Buffer
	if err := image.EmitReadmemh(&memBuf, img); err != nil {
		t.Fatalf("EmitReadmemh: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(memBuf.String()), "\n")
	if len(lines) < 4 {
		t.Errorf("expected several .mem lines, got %d", len(lines))
	}
	if len(lines[0]) != 8 {
		t.Errorf(".mem line 0 = %q (expected 8 hex digits)", lines[0])
	}
}

// ─── Forward + backward branches in the same function ────────────────────

func TestForwardAndBackwardBranchesCoexist(t *testing.T) {
	src := `
.text
main:
    addi t0, x0, 5      ; PC=0
    j    body           ; PC=4 → +8
    addi t0, t0, 1      ; PC=8 (skipped)
body:
    addi t0, t0, -1     ; PC=12
    bnez t0, body       ; PC=16 → -4
    ret                 ; PC=20
`
	img := fullLink(t, nil, struct{ name, src string }{"t.s", src})

	// j +8 = jal x0, +8 = 0x0080006F
	if w := binary.LittleEndian.Uint32(img.TextData[4:8]); w != 0x0080006F {
		t.Errorf("j = 0x%08X want 0x0080006F", w)
	}
	// bnez t0, -4 → bne x5, x0, -4 = 0xFE029EE3
	if w := binary.LittleEndian.Uint32(img.TextData[16:20]); w != 0xFE029EE3 {
		t.Errorf("bnez = 0x%08X want 0xFE029EE3", w)
	}
}

// ─── Branch overflow is rejected ─────────────────────────────────────────

func TestBranchOverflowIsRejected(t *testing.T) {
	// Pad with > 4 KiB worth of nops between the branch and target.
	var sb strings.Builder
	sb.WriteString(".text\nstart:\nbeq x1, x2, far\n")
	for i := 0; i < 1100; i++ { // 1100 × 4 = 4400 bytes > 4 KiB
		sb.WriteString("nop\n")
	}
	sb.WriteString("far:\nnop\n")

	defer func() {
		// We only care that *something* errored; success → fail.
	}()

	m := asmFile(t, "ovf.s", sb.String())
	img, err := linker.Link([]linker.Input{{Path: "ovf.s", Module: m}}, script.Default())
	if err != nil {
		return // could fail at link layout instead — also fine
	}
	if err := rl.Apply(img); err == nil {
		t.Fatalf("expected branch-overflow relocation error, got success")
	}
}

// ─── Duplicate symbol diagnostic ─────────────────────────────────────────

func TestDuplicateLocalLabelInOneFile(t *testing.T) {
	src := `
.text
foo:
    nop
foo:
    nop
`
	toks, _ := lexer.Lex("dup.s", []byte(src))
	stmts, _ := parser.Parse("dup.s", toks)
	if _, err := encoder.Encode(stmts); err == nil {
		t.Fatal("expected duplicate label error in one file")
	}
}

// ─── Undefined extern across files ───────────────────────────────────────

func TestUndefinedExternAcrossFiles(t *testing.T) {
	src := `
.text
.global _start
_start:
    call missing
    ret
`
	m := asmFile(t, "u.s", src)
	if _, err := linker.Link([]linker.Input{{Path: "u.s", Module: m}}, script.Default()); err == nil {
		t.Fatal("expected undefined-symbol error")
	}
}

// ─── Data section with absolute reference ────────────────────────────────

func TestDataAbsoluteReference(t *testing.T) {
	src := `
.global tab
.data
tab:
    .word entry        ; will be patched to entry's absolute address
.text
.global entry
entry:
    addi x0, x0, 0
`
	img := fullLink(t, nil, struct{ name, src string }{"abs.s", src})
	// entry is at 0 in default layout.
	w := binary.LittleEndian.Uint32(img.DataData[0:4])
	if w != 0 {
		t.Errorf("tab[0] = 0x%X, expected 0 (entry's absolute address)", w)
	}
}

// ─── HI20/LO12 round-trip via la pseudo ──────────────────────────────────

func TestLAPseudoCorrectness(t *testing.T) {
	src := `
.text
main:
    la a0, msg          ; PC 0,4
    ret                 ; PC 8
.data
msg:
    .ascii "hi"
`
	img := fullLink(t, nil, struct{ name, src string }{"la.s", src})
	// Verify that the program would actually load msg's address by
	// emulating the auipc + addi math.
	auipc := binary.LittleEndian.Uint32(img.TextData[0:4])
	addi := binary.LittleEndian.Uint32(img.TextData[4:8])

	// Decode auipc imm[31:12]
	hi20 := auipc & 0xFFFFF000
	// Decode addi imm[11:0] sign-extended
	rawLo := int32(addi) >> 20
	lo12 := rawLo // sign-extended

	// PC of auipc is 0 (text base 0)
	computed := uint32(int32(hi20) + lo12)

	msgAddr := img.DataBase
	if computed != msgAddr {
		t.Errorf("la a0,msg computes 0x%X, want 0x%X (msg)", computed, msgAddr)
	}
}
