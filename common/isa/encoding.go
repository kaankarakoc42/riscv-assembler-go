package isa

// This file contains the six primitive instruction-format encoders for RV32I.
// Each one packs its arguments into a 32-bit machine word.
//
// Notation: bit 0 is least significant. Field positions follow the official
// RISC-V User-Level ISA Manual, Volume I, Chapter 2.
//
// Crucially, the B-type and J-type immediates are *scrambled*. We rebuild them
// here, and the relocation engine will splice patched immediates into existing
// instructions using the same bit layout.

// EncodeR — register-register ALU ops.
//
//   31     25 24   20 19   15 14   12 11   7 6     0
//  ┌────────┬────────┬────────┬───────┬──────┬──────┐
//  │ funct7 │  rs2   │  rs1   │funct3 │  rd  │opcode│
//  └────────┴────────┴────────┴───────┴──────┴──────┘
func EncodeR(opcode, rd, funct3, rs1, rs2, funct7 uint32) uint32 {
	return (funct7&0x7F)<<25 |
		(rs2&0x1F)<<20 |
		(rs1&0x1F)<<15 |
		(funct3&0x07)<<12 |
		(rd&0x1F)<<7 |
		(opcode & 0x7F)
}

// EncodeI — immediate ALU, loads, jalr.
//
//   31         20 19   15 14   12 11   7 6     0
//  ┌─────────────┬────────┬───────┬──────┬──────┐
//  │  imm[11:0]  │  rs1   │funct3 │  rd  │opcode│
//  └─────────────┴────────┴───────┴──────┴──────┘
//
// imm is treated as a 12-bit two's-complement signed value. Caller is expected
// to pass the *int32* in `imm` truncated to 12 bits. We mask, but bounds
// checking happens upstream.
func EncodeI(opcode, rd, funct3, rs1 uint32, imm int32) uint32 {
	return (uint32(imm)&0xFFF)<<20 |
		(rs1&0x1F)<<15 |
		(funct3&0x07)<<12 |
		(rd&0x1F)<<7 |
		(opcode & 0x7F)
}

// EncodeS — stores.
//
//   31     25 24   20 19   15 14   12 11   7 6     0
//  ┌────────┬────────┬────────┬───────┬──────┬──────┐
//  │imm[11:5]│  rs2  │  rs1   │funct3 │imm4:0│opcode│
//  └────────┴────────┴────────┴───────┴──────┴──────┘
func EncodeS(opcode, funct3, rs1, rs2 uint32, imm int32) uint32 {
	uimm := uint32(imm) & 0xFFF
	return ((uimm>>5)&0x7F)<<25 |
		(rs2&0x1F)<<20 |
		(rs1&0x1F)<<15 |
		(funct3&0x07)<<12 |
		(uimm&0x1F)<<7 |
		(opcode & 0x7F)
}

// EncodeB — conditional branches. The immediate is signed, multiple of 2,
// scrambled across the instruction word:
//
//   inst[31]    = imm[12]
//   inst[30:25] = imm[10:5]
//   inst[11:8]  = imm[4:1]
//   inst[7]     = imm[11]
//   imm[0]      = 0  (always — branches are 2-byte aligned)
//
// `imm` is the byte offset from the branch *PC* to the target.
// 13-bit signed range: -4096 .. +4094.
func EncodeB(opcode, funct3, rs1, rs2 uint32, imm int32) uint32 {
	uimm := uint32(imm) & 0x1FFE // 13-bit, bottom bit cleared
	bit12 := (uimm >> 12) & 0x1
	bit11 := (uimm >> 11) & 0x1
	bits10_5 := (uimm >> 5) & 0x3F
	bits4_1 := (uimm >> 1) & 0xF
	return (bit12 << 31) |
		(bits10_5 << 25) |
		(rs2&0x1F)<<20 |
		(rs1&0x1F)<<15 |
		(funct3&0x07)<<12 |
		(bits4_1 << 8) |
		(bit11 << 7) |
		(opcode & 0x7F)
}

// EncodeU — lui, auipc. The immediate is the upper 20 bits of a 32-bit value;
// hardware shifts it left by 12 and sign-extends. Caller hands us the
// already-shifted, fully-formed 20-bit value in the low 20 bits of `imm`.
//
//   31              12 11   7 6     0
//  ┌───────────────────┬──────┬──────┐
//  │     imm[31:12]    │  rd  │opcode│
//  └───────────────────┴──────┴──────┘
func EncodeU(opcode, rd uint32, imm uint32) uint32 {
	return (imm & 0xFFFFF000) |
		(rd&0x1F)<<7 |
		(opcode & 0x7F)
}

// EncodeJ — jal. Immediate is signed, 21-bit, multiple of 2:
//
//   inst[31]    = imm[20]
//   inst[30:21] = imm[10:1]
//   inst[20]    = imm[11]
//   inst[19:12] = imm[19:12]
//   imm[0]      = 0
//
// `imm` is the byte offset from the jal PC to the target.
// 21-bit signed range: -1MiB .. +1MiB-2.
func EncodeJ(opcode, rd uint32, imm int32) uint32 {
	uimm := uint32(imm) & 0x1FFFFE
	bit20 := (uimm >> 20) & 0x1
	bits19_12 := (uimm >> 12) & 0xFF
	bit11 := (uimm >> 11) & 0x1
	bits10_1 := (uimm >> 1) & 0x3FF
	return (bit20 << 31) |
		(bits10_1 << 21) |
		(bit11 << 20) |
		(bits19_12 << 12) |
		(rd&0x1F)<<7 |
		(opcode & 0x7F)
}
