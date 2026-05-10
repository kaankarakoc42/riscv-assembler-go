package isa

import "testing"

// These tests pin down the bit-exact encoding of a few well-known instructions
// against hand-computed expected words. If any of these fails, the rest of the
// toolchain is broken.

func TestEncodeR_AddX1X2X3(t *testing.T) {
	// add x1, x2, x3   →   0x003100B3
	// funct7=0  rs2=3   rs1=2   funct3=0  rd=1   opcode=0x33
	got := EncodeR(0x33, 1, 0, 2, 3, 0)
	want := uint32(0x003100B3)
	if got != want {
		t.Fatalf("add x1,x2,x3 = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeR_SubX5X6X7(t *testing.T) {
	// sub x5, x6, x7   →   funct7=0x20, rs2=7, rs1=6, funct3=0, rd=5, op=0x33
	// = 01000000 0111 00110 000 00101 0110011 = 0x407302B3
	got := EncodeR(0x33, 5, 0, 6, 7, 0x20)
	want := uint32(0x407302B3)
	if got != want {
		t.Fatalf("sub = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeI_AddiX1X0_5(t *testing.T) {
	// addi x1, x0, 5   →   0x00500093
	got := EncodeI(0x13, 1, 0, 0, 5)
	want := uint32(0x00500093)
	if got != want {
		t.Fatalf("addi = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeI_AddiNegativeImm(t *testing.T) {
	// addi x5, x6, -1  →   imm=0xFFF, funct3=0, rs1=6, rd=5, op=0x13
	// = 1111_1111_1111 00110 000 00101 0010011 = 0xFFF30293
	got := EncodeI(0x13, 5, 0, 6, -1)
	want := uint32(0xFFF30293)
	if got != want {
		t.Fatalf("addi -1 = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeS_SwX5_8X2(t *testing.T) {
	// sw x5, 8(x2)   →   imm=8, rs2=5, rs1=2, funct3=2, op=0x23
	// imm[11:5]=0, imm[4:0]=8 → 0000000 00101 00010 010 01000 0100011 = 0x00512423
	got := EncodeS(0x23, 2, 2, 5, 8)
	want := uint32(0x00512423)
	if got != want {
		t.Fatalf("sw = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeS_NegativeOffset(t *testing.T) {
	// sw x1, -4(x2)  → imm=-4 (0xFFC), rs2=1, rs1=2, funct3=2, op=0x23
	// imm[11:5]=0x7F, imm[4:0]=0x1C
	// = 1111111 00001 00010 010 11100 0100011 = 0xFE112E23
	got := EncodeS(0x23, 2, 2, 1, -4)
	want := uint32(0xFE112E23)
	if got != want {
		t.Fatalf("sw -4 = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeB_BeqForward8(t *testing.T) {
	// beq x1, x2, +8 (offset 8) → funct3=0, rs1=1, rs2=2, op=0x63
	// imm[12]=0 imm[11]=0 imm[10:5]=0 imm[4:1]=4 imm[0]=0
	// 0 000000 00010 00001 000 0100 0 1100011 = 0x00208463
	got := EncodeB(0x63, 0, 1, 2, 8)
	want := uint32(0x00208463)
	if got != want {
		t.Fatalf("beq +8 = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeB_BneBackward(t *testing.T) {
	// bne x5, x6, -4  → imm=-4 (0x1FFC in 13-bit two's comp)
	// imm[12]=1 imm[11]=1 imm[10:5]=0x3F imm[4:1]=0xE imm[0]=0
	// funct3=1 rs1=5 rs2=6 op=0x63
	// 1 111111 00110 00101 001 1110 1 1100011 = 0xFE629EE3
	got := EncodeB(0x63, 1, 5, 6, -4)
	want := uint32(0xFE629EE3)
	if got != want {
		t.Fatalf("bne -4 = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeU_LuiX1_0x12345(t *testing.T) {
	// lui x1, 0x12345  → imm=0x12345000, rd=1, op=0x37
	// = 0x123450B7
	got := EncodeU(0x37, 1, 0x12345000)
	want := uint32(0x123450B7)
	if got != want {
		t.Fatalf("lui = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeJ_JalX1_Forward16(t *testing.T) {
	// jal x1, +16 (i.e., target = PC+16). imm=16
	// imm[20]=0 imm[19:12]=0 imm[11]=0 imm[10:1]=8
	// rd=1, op=0x6F
	// 0 0000001000 0 00000000 00001 1101111 = 0x010000EF
	got := EncodeJ(0x6F, 1, 16)
	want := uint32(0x010000EF)
	if got != want {
		t.Fatalf("jal = 0x%08X want 0x%08X", got, want)
	}
}

func TestEncodeJ_Backward(t *testing.T) {
	// jal x0, -8   →   imm=-8 (0x1FFFF8 in 21-bit two's comp)
	// imm[20]=1 imm[19:12]=0xFF imm[11]=1 imm[10:1]=0x3FC
	// = 1 1111111100 1 11111111 00000 1101111 = 0xFF9FF06F
	got := EncodeJ(0x6F, 0, -8)
	want := uint32(0xFF9FF06F)
	if got != want {
		t.Fatalf("jal -8 = 0x%08X want 0x%08X", got, want)
	}
}

func TestAssemble_Ecall(t *testing.T) {
	op, _ := LookupOp("ecall")
	got, err := Assemble(op, Operands{})
	if err != nil {
		t.Fatal(err)
	}
	if got != 0x00000073 {
		t.Fatalf("ecall = 0x%08X want 0x00000073", got)
	}
}

func TestAssemble_Ebreak(t *testing.T) {
	op, _ := LookupOp("ebreak")
	got, err := Assemble(op, Operands{})
	if err != nil {
		t.Fatal(err)
	}
	if got != 0x00100073 {
		t.Fatalf("ebreak = 0x%08X want 0x00100073", got)
	}
}

func TestAssemble_SlliShamt(t *testing.T) {
	op, _ := LookupOp("slli")
	// slli x1, x2, 5  → funct7=0, shamt=5, rs1=2, funct3=1, rd=1, op=0x13
	// = 0000000 00101 00010 001 00001 0010011 = 0x00511093
	got, err := Assemble(op, Operands{Rd: 1, Rs1: 2, Shamt: 5})
	if err != nil {
		t.Fatal(err)
	}
	want := uint32(0x00511093)
	if got != want {
		t.Fatalf("slli = 0x%08X want 0x%08X", got, want)
	}
}

func TestAssemble_BranchAlignmentError(t *testing.T) {
	op, _ := LookupOp("beq")
	_, err := Assemble(op, Operands{Rs1: 1, Rs2: 2, Imm: 7})
	if err == nil {
		t.Fatal("expected error for unaligned branch offset")
	}
}

func TestAssemble_AddiOverflowError(t *testing.T) {
	op, _ := LookupOp("addi")
	_, err := Assemble(op, Operands{Rd: 1, Rs1: 2, Imm: 4096})
	if err == nil {
		t.Fatal("expected error for 12-bit overflow")
	}
}

func TestRegister_AbiAliases(t *testing.T) {
	cases := map[string]uint32{
		"zero": 0, "ra": 1, "sp": 2, "gp": 3, "tp": 4,
		"t0": 5, "a0": 10, "s0": 8, "fp": 8, "t6": 31,
	}
	for name, want := range cases {
		got, ok := Reg(name)
		if !ok || got != want {
			t.Fatalf("Reg(%q)=%d ok=%v, want %d", name, got, ok, want)
		}
	}
}
