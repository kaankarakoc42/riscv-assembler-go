// Package reloc applies relocations to a fully-laid-out Image.
//
// See ../../docs/relocation.md for the bit-level rationale of every type.
package reloc

import (
	"encoding/binary"
	"fmt"

	"github.com/edu/rvtoolchain/common/obj"
	rt "github.com/edu/rvtoolchain/common/reloc"
	"github.com/edu/rvtoolchain/linker"
)

// Apply walks every input module's relocation table and patches the merged
// image bytes in place.
//
// For each relocation we need:
//   - P: the absolute patch-site address (input section base + reloc offset)
//   - S: the absolute address of the referenced symbol
//   - A: the addend
//
// We dispatch by reloc.Type and reuse the same scrambled-bit encodings that
// the assembler used in common/isa/encoding.go.
func Apply(img *linker.Image) error {
	for ii, in := range img.Inputs {
		for ri := range in.Module.Relocs {
			r := in.Module.Relocs[ri]
			if err := applyOne(img, uint32(ii), in.Module, r); err != nil {
				return fmt.Errorf("reloc[%d] in %s: %v", ri, in.Path, err)
			}
		}
	}
	return nil
}

func applyOne(img *linker.Image, inputIdx uint32, mod *obj.Module, r obj.Reloc) error {
	// Resolve P — the patch site.
	place, ok := img.PlacementFor(inputIdx, r.SectionIdx)
	if !ok {
		return fmt.Errorf("reloc references unplaced section %d", r.SectionIdx)
	}
	P := place.Base + r.Offset

	// Resolve S — the target symbol's absolute address.
	if r.SymIdx >= uint32(len(mod.Symbols)) {
		return fmt.Errorf("reloc symIdx %d out of range", r.SymIdx)
	}
	sym := mod.Symbols[r.SymIdx]
	S, err := resolveSymbolAddr(img, inputIdx, sym)
	if err != nil {
		return err
	}
	A := r.Addend

	// Slice into the merged buffer holding the patch site.
	buf, off, err := img.PatchSlice(place, r.Offset)
	if err != nil {
		return err
	}

	switch r.Type {
	case rt.R_RV32_NONE:
		return nil

	case rt.R_RV32_32:
		// V = S + A, written as a 32-bit absolute value.
		V := uint32(int64(S) + int64(A))
		binary.LittleEndian.PutUint32(buf[off:], V)
		return nil

	case rt.R_RV32_BRANCH:
		V := int64(S) + int64(A) - int64(P)
		if V&1 != 0 {
			return fmt.Errorf("R_RV32_BRANCH: target 0x%X not 2-byte aligned", V)
		}
		if V < -4096 || V > 4094 {
			return fmt.Errorf("R_RV32_BRANCH: offset %d out of ±4 KiB range", V)
		}
		w := binary.LittleEndian.Uint32(buf[off:])
		w = (w &^ bMask()) | encodeBImm(int32(V))
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil

	case rt.R_RV32_JAL:
		V := int64(S) + int64(A) - int64(P)
		if V&1 != 0 {
			return fmt.Errorf("R_RV32_JAL: target 0x%X not 2-byte aligned", V)
		}
		if V < -(1<<20) || V > (1<<20)-2 {
			return fmt.Errorf("R_RV32_JAL: offset %d out of ±1 MiB", V)
		}
		w := binary.LittleEndian.Uint32(buf[off:])
		w = (w &^ jMask()) | encodeJImm(int32(V))
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil

	case rt.R_RV32_HI20:
		V := uint32(int64(S) + int64(A))
		hi, _ := splitHILO(V)
		w := binary.LittleEndian.Uint32(buf[off:])
		w = (w &^ uMask()) | (hi & 0xFFFFF000)
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil

	case rt.R_RV32_LO12_I:
		V := uint32(int64(S) + int64(A))
		_, lo := splitHILO(V)
		w := binary.LittleEndian.Uint32(buf[off:])
		// Mask out imm[11:0] = bits 31:20.
		w = (w &^ (uint32(0xFFF) << 20)) | ((lo & 0xFFF) << 20)
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil

	case rt.R_RV32_LO12_S:
		V := uint32(int64(S) + int64(A))
		_, lo := splitHILO(V)
		w := binary.LittleEndian.Uint32(buf[off:])
		// S-type imm bits: [31:25]=imm[11:5], [11:7]=imm[4:0]
		mask := uint32(0xFE000F80)
		w = (w &^ mask) |
			((lo>>5)&0x7F)<<25 |
			((lo)&0x1F)<<7
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil

	case rt.R_RV32_PCREL_HI20:
		// Vfull = S + A - P, then split into HI20/LO12.
		// We patch HI20 only; the paired PCREL_LO12_I will patch its own site.
		Vfull := int64(S) + int64(A) - int64(P)
		hi, _ := splitHILO(uint32(Vfull))
		w := binary.LittleEndian.Uint32(buf[off:])
		w = (w &^ uMask()) | (hi & 0xFFFFF000)
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil

	case rt.R_RV32_PCREL_LO12_I:
		// Find the paired auipc: it must be a previous PCREL_HI20 with the
		// same symbol, in the same section, in this same input. The AUIPC's
		// PC is what we use for the (S - PC) split — NOT the addi's PC.
		auipcPC, err := findPairedAuipc(img, inputIdx, mod, r, place)
		if err != nil {
			return err
		}
		Vfull := int64(S) + int64(A) - int64(auipcPC)
		_, lo := splitHILO(uint32(Vfull))
		w := binary.LittleEndian.Uint32(buf[off:])
		w = (w &^ (uint32(0xFFF) << 20)) | ((lo & 0xFFF) << 20)
		binary.LittleEndian.PutUint32(buf[off:], w)
		return nil
	}
	return fmt.Errorf("unsupported reloc type %s", r.Type)
}

// resolveSymbolAddr returns the final absolute address for a symbol,
// dispatching between local definition (use placement) and external (look
// up in the global symbol table).
func resolveSymbolAddr(img *linker.Image, inputIdx uint32, sym obj.Symbol) (uint32, error) {
	if sym.Bind == obj.BindExtern || sym.SectionIdx == obj.SectionExtern {
		g, ok := img.Globals[sym.Name]
		if !ok {
			return 0, fmt.Errorf("undefined external %q", sym.Name)
		}
		return g.Address, nil
	}
	return img.AbsForSymbol(inputIdx, sym)
}

// findPairedAuipc walks backwards through the reloc list of mod looking for
// a PCREL_HI20 with the same SymIdx in the same SectionIdx. The AUIPC is at
// place.Base + that_reloc.Offset.
//
// This is the simplest correct strategy. Real toolchains require the
// programmer to use a label that names the auipc and have the LO12 reloc
// reference *that label*; we sidestep the label requirement by pairing on
// symbol identity (which our pseudo-instruction expander always preserves).
func findPairedAuipc(img *linker.Image, inputIdx uint32, mod *obj.Module, lo obj.Reloc, place linker.PlacedSection) (uint32, error) {
	var bestOff uint32
	var found bool
	for _, r := range mod.Relocs {
		if r.Type != rt.R_RV32_PCREL_HI20 {
			continue
		}
		if r.SectionIdx != lo.SectionIdx {
			continue
		}
		if r.SymIdx != lo.SymIdx {
			continue
		}
		if r.Offset >= lo.Offset {
			// Must be earlier in the section.
			continue
		}
		if !found || r.Offset > bestOff {
			bestOff = r.Offset
			found = true
		}
	}
	if !found {
		return 0, fmt.Errorf("no paired R_RV32_PCREL_HI20 found for LO12 site at 0x%X", place.Base+lo.Offset)
	}
	return place.Base + bestOff, nil
}

// ─────────────────────────────────────────────────────────────────────────
// Bit-level helpers (mirrored against common/isa/encoding.go).
// ─────────────────────────────────────────────────────────────────────────

// bMask returns the union of bits patched by R_RV32_BRANCH:
// inst[31], inst[30:25], inst[11:8], inst[7].
func bMask() uint32 { return (uint32(1) << 31) | (uint32(0x3F) << 25) | (uint32(0xF) << 8) | (uint32(1) << 7) }

func encodeBImm(v int32) uint32 {
	uimm := uint32(v) & 0x1FFE
	bit12 := (uimm >> 12) & 0x1
	bit11 := (uimm >> 11) & 0x1
	bits10_5 := (uimm >> 5) & 0x3F
	bits4_1 := (uimm >> 1) & 0xF
	return (bit12 << 31) | (bits10_5 << 25) | (bits4_1 << 8) | (bit11 << 7)
}

// jMask returns the union of bits patched by R_RV32_JAL:
// inst[31], inst[30:21], inst[20], inst[19:12].
func jMask() uint32 {
	return (uint32(1) << 31) | (uint32(0x3FF) << 21) | (uint32(1) << 20) | (uint32(0xFF) << 12)
}

func encodeJImm(v int32) uint32 {
	uimm := uint32(v) & 0x1FFFFE
	bit20 := (uimm >> 20) & 0x1
	bits19_12 := (uimm >> 12) & 0xFF
	bit11 := (uimm >> 11) & 0x1
	bits10_1 := (uimm >> 1) & 0x3FF
	return (bit20 << 31) | (bits10_1 << 21) | (bit11 << 20) | (bits19_12 << 12)
}

// uMask covers inst[31:12] = U-type immediate.
func uMask() uint32 { return 0xFFFFF000 }

// splitHILO mirrors encoder/pseudo.go: HI20 carry-corrected for sign-extended LO12.
func splitHILO(v uint32) (hi, lo uint32) {
	lo = v & 0xFFF
	hi = v >> 12
	if lo&0x800 != 0 {
		hi++
	}
	return hi << 12, lo
}
