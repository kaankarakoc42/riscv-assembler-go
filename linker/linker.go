// Package linker performs multi-file linking of RVOB1 objects into a flat
// memory image suitable for FPGA BRAM.
//
// Pipeline (see also docs/architecture.md):
//
//   1. Load all input modules (obj.Read).
//   2. Layout: assign each input section a final base address.
//      All .text inputs are concatenated, then all .data inputs.
//   3. Resolve symbols into a global symbol table.
//   4. Apply relocations (linker/reloc package).
//   5. Emit raw + hex + mem images (linker/image package).
package linker

import (
	"fmt"

	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/linker/script"
)

// Input is one loaded object file plus its origin path (for diagnostics).
type Input struct {
	Path   string
	Module *obj.Module
}

// PlacedSection records the absolute base address chosen for one input
// section.
//
//   - InputIdx: index in the original Inputs slice.
//   - LocalIdx: index of the section within that module (e.g. 0 for .text).
//   - Base:     final absolute address (in bytes, in the merged image).
type PlacedSection struct {
	InputIdx uint32
	LocalIdx uint8
	Name     string
	Base     uint32
	Size     uint32
}

// GlobalSym is one entry of the linker's global symbol table.
//
// Address is the *final absolute* address (post-layout, post-section-base).
type GlobalSym struct {
	Name     string
	Address  uint32
	Defining int // input index that defines it (-1 for unresolved)
}

// Image is the linker's working state and the structured result returned to
// the emitter.
type Image struct {
	Script *script.Script

	// Final memory layout.
	TextBase uint32
	TextData []byte
	DataBase uint32
	DataData []byte

	// Per-input section placement.
	Placements []PlacedSection

	// Global symbol table (post-resolve).
	Globals map[string]GlobalSym

	// Optional: keep the inputs around so the relocation pass can walk
	// each module's relocs without re-parsing.
	Inputs []Input
}

// Link runs the front half of the pipeline: load + layout + resolve. The
// caller then runs the relocation pass and the emitter passes.
//
// Splitting it this way makes each phase independently testable.
func Link(inputs []Input, sc *script.Script) (*Image, error) {
	if sc == nil {
		sc = script.Default()
	}
	img := &Image{Script: sc, Inputs: inputs, Globals: map[string]GlobalSym{}}

	if err := layout(img); err != nil {
		return nil, err
	}
	if err := resolve(img); err != nil {
		return nil, err
	}
	return img, nil
}

// ─────────────────────────────────────────────────────────────────────────
// Layout pass
// ─────────────────────────────────────────────────────────────────────────

// layout decides the final base address of every input section.
//
//   * All .text sections from all inputs are concatenated, in input order,
//     starting at TextBase.
//   * All .data sections are concatenated, in input order, starting at
//     DataBase. DataBase is either RomBase + len(text) (if data_at = rom) or
//     RamBase (if data_at = ram).
//   * .text and .data are 4-byte aligned. Each input section is also rounded
//     up to a multiple of 4 to keep the next instruction word aligned.
func layout(img *Image) error {
	sc := img.Script

	textBase := sc.RomBase
	if sc.TextAt == "ram" {
		textBase = sc.RamBase
	}
	img.TextBase = textBase

	// First pass: place all .text sections and accumulate size.
	cur := textBase
	for ii, in := range img.Inputs {
		for li, s := range in.Module.Sections {
			if s.Name != ".text" {
				continue
			}
			img.Placements = append(img.Placements, PlacedSection{
				InputIdx: uint32(ii),
				LocalIdx: uint8(li),
				Name:     ".text",
				Base:     cur,
				Size:     uint32(len(s.Data)),
			})
			cur += alignUp(uint32(len(s.Data)), 4)
		}
	}
	textEnd := cur

	// Now place .data.
	var dataBase uint32
	if sc.DataAt == "ram" {
		dataBase = sc.RamBase
	} else {
		dataBase = alignUp(textEnd, 4)
	}
	img.DataBase = dataBase

	cur = dataBase
	for ii, in := range img.Inputs {
		for li, s := range in.Module.Sections {
			if s.Name != ".data" {
				continue
			}
			img.Placements = append(img.Placements, PlacedSection{
				InputIdx: uint32(ii),
				LocalIdx: uint8(li),
				Name:     ".data",
				Base:     cur,
				Size:     uint32(len(s.Data)),
			})
			cur += alignUp(uint32(len(s.Data)), 4)
		}
	}
	dataEnd := cur

	// Allocate the merged buffers, zero-filled by Go default.
	img.TextData = make([]byte, textEnd-textBase)
	img.DataData = make([]byte, dataEnd-dataBase)

	// Splat the bytes from each input into the right slot.
	for _, p := range img.Placements {
		src := img.Inputs[p.InputIdx].Module.Sections[p.LocalIdx].Data
		var dst []byte
		var off uint32
		if p.Name == ".text" {
			dst = img.TextData
			off = p.Base - img.TextBase
		} else {
			dst = img.DataData
			off = p.Base - img.DataBase
		}
		copy(dst[off:off+uint32(len(src))], src)
	}

	// Bounds checks.
	if uint64(len(img.TextData)) > uint64(sc.RomSize) && sc.TextAt == "rom" {
		return fmt.Errorf("link: .text (%d B) exceeds ROM size (%d B)", len(img.TextData), sc.RomSize)
	}
	if sc.DataAt == "rom" {
		if uint64(textEnd-textBase)+uint64(len(img.DataData)) > uint64(sc.RomSize) {
			return fmt.Errorf("link: .text + .data exceeds ROM size (%d B)", sc.RomSize)
		}
	} else {
		if uint64(len(img.DataData)) > uint64(sc.RamSize) {
			return fmt.Errorf("link: .data (%d B) exceeds RAM size (%d B)", len(img.DataData), sc.RamSize)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────
// Symbol resolution pass
// ─────────────────────────────────────────────────────────────────────────

// resolve builds the global symbol table.
//
//   * LOCAL  symbols stay private to their module (still needed for
//     relocations within that module).
//   * GLOBAL symbols enter the global map.
//     Two GLOBAL definitions with the same name → error.
//   * EXTERN symbols must match exactly one GLOBAL definition; we record
//     that mapping for the relocation pass.
func resolve(img *Image) error {
	for ii, in := range img.Inputs {
		for _, sym := range in.Module.Symbols {
			if sym.Bind != obj.BindGlobal {
				continue
			}
			abs, err := img.AbsForSymbol(uint32(ii), sym)
			if err != nil {
				return fmt.Errorf("%s: %v", in.Path, err)
			}
			if existing, dup := img.Globals[sym.Name]; dup {
				return fmt.Errorf("link: duplicate global symbol %q (defined in %s and %s)",
					sym.Name, img.Inputs[existing.Defining].Path, in.Path)
			}
			img.Globals[sym.Name] = GlobalSym{
				Name:     sym.Name,
				Address:  abs,
				Defining: ii,
			}
		}
	}

	// Verify all externs resolve.
	for _, in := range img.Inputs {
		for _, sym := range in.Module.Symbols {
			if sym.Bind != obj.BindExtern {
				continue
			}
			if _, ok := img.Globals[sym.Name]; !ok {
				return fmt.Errorf("link: undefined reference to %q (used in %s)", sym.Name, in.Path)
			}
		}
	}
	return nil
}

// AbsForSymbol returns the final absolute address of a symbol defined in
// input `inputIdx`.
//
// For LOCAL/GLOBAL symbols this is `placement.Base + sym.Value`.
// For EXTERN symbols, the caller should look up via Globals and use the
// returned Address.
func (img *Image) AbsForSymbol(inputIdx uint32, sym obj.Symbol) (uint32, error) {
	if sym.Bind == obj.BindExtern || sym.SectionIdx == obj.SectionExtern {
		return 0, fmt.Errorf("AbsForSymbol called on extern %q", sym.Name)
	}
	for _, p := range img.Placements {
		if p.InputIdx == inputIdx && p.LocalIdx == sym.SectionIdx {
			return p.Base + sym.Value, nil
		}
	}
	return 0, fmt.Errorf("no placement for symbol %q (section %d of input %d)",
		sym.Name, sym.SectionIdx, inputIdx)
}

// PlacementFor returns the placement of an input section (if present).
func (img *Image) PlacementFor(inputIdx uint32, localIdx uint8) (PlacedSection, bool) {
	for _, p := range img.Placements {
		if p.InputIdx == inputIdx && p.LocalIdx == localIdx {
			return p, true
		}
	}
	return PlacedSection{}, false
}

// PatchSlice returns a writable slice into the merged image that contains
// the bytes of `place`. The returned (buf, off) pair satisfies the property:
//
//	buf[off : off+4] is the 32-bit word at place.Base + reloff
//
// `reloff` is the section-relative offset stored in the relocation record.
func (img *Image) PatchSlice(place PlacedSection, reloff uint32) ([]byte, uint32, error) {
	if reloff+4 > place.Size {
		return nil, 0, fmt.Errorf("relocation offset %d exceeds section size %d", reloff, place.Size)
	}
	switch place.Name {
	case ".text":
		base := place.Base - img.TextBase
		return img.TextData, base + reloff, nil
	case ".data":
		base := place.Base - img.DataBase
		return img.DataData, base + reloff, nil
	}
	return nil, 0, fmt.Errorf("unsupported section name for relocation: %q", place.Name)
}

// alignUp rounds n up to a multiple of `to` (which must be a power of two).
func alignUp(n, to uint32) uint32 {
	mask := to - 1
	return (n + mask) &^ mask
}
