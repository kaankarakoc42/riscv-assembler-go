// Package image emits the linked program in three flavors:
//
//   - raw .bin — flat little-endian bytes, ROM image
//   - Intel HEX (.hex) — universally accepted by flashers
//   - $readmemh-compatible .mem — one 32-bit word per line, for FPGA BRAM
//
// All three are derived from the same conceptual structure: a flat byte
// array starting at the ROM base address. The linker places .text first
// and .data immediately after (when data_at = rom). When data_at = ram,
// .data is emitted as a separate file (program.data.mem) so the FPGA can
// initialise its data BRAM from a different file.
package image

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/edu/rvtoolchain/linker"
)

// EmitAll writes program.bin, program.hex, and program.mem (and possibly
// program.data.bin / .mem if data_at = ram).
//
// outBase is the path prefix; e.g. outBase="build/blink" produces:
//
//	build/blink.bin
//	build/blink.hex
//	build/blink.mem
//
// and (if data lives in RAM) build/blink.data.mem.
func EmitAll(img *linker.Image, outBase string) error {
	if err := emitFile(outBase+".bin", func(w io.Writer) error {
		return EmitBin(w, img)
	}); err != nil {
		return err
	}
	if err := emitFile(outBase+".hex", func(w io.Writer) error {
		return EmitIntelHex(w, img)
	}); err != nil {
		return err
	}
	if err := emitFile(outBase+".mem", func(w io.Writer) error {
		return EmitReadmemh(w, img)
	}); err != nil {
		return err
	}
	if img.Script.DataAt == "ram" && len(img.DataData) > 0 {
		if err := emitFile(outBase+".data.mem", func(w io.Writer) error {
			return emitReadmemhBytes(w, img.DataData)
		}); err != nil {
			return err
		}
	}
	return nil
}

func emitFile(path string, fn func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	if err := fn(bw); err != nil {
		return err
	}
	return bw.Flush()
}

// ─────────────────────────────────────────────────────────────────────────
// .bin — flat ROM image
// ─────────────────────────────────────────────────────────────────────────
//
// We emit, starting at offset 0:
//
//   bytes 0..len(text)-1     : .text
//   bytes len(text)..        : .data  (only if data_at = rom)
//
// For data_at = ram, .data is emitted separately because it lives in a
// different memory.
func EmitBin(w io.Writer, img *linker.Image) error {
	if _, err := w.Write(img.TextData); err != nil {
		return err
	}
	if img.Script.DataAt == "rom" {
		gap := alignedGap(img.TextBase, len(img.TextData), img.DataBase)
		if gap > 0 {
			pad := make([]byte, gap)
			if _, err := w.Write(pad); err != nil {
				return err
			}
		}
		if _, err := w.Write(img.DataData); err != nil {
			return err
		}
	}
	return nil
}

func alignedGap(textBase uint32, textLen int, dataBase uint32) int {
	have := uint64(textBase) + uint64(textLen)
	want := uint64(dataBase)
	if want < have {
		return 0
	}
	return int(want - have)
}

// ─────────────────────────────────────────────────────────────────────────
// .mem — $readmemh, 32-bit words, one per line
// ─────────────────────────────────────────────────────────────────────────
//
// `mem` is a byte-addressed memory in our software model, but
// FPGA BRAMs are usually word-addressed. We emit big-step 4-byte words.
// Each line is the hex of the 32-bit instruction or data word at that
// address, MSB on the left, no "0x" prefix, padded to 8 hex digits.
func EmitReadmemh(w io.Writer, img *linker.Image) error {
	if err := emitReadmemhBytes(w, img.TextData); err != nil {
		return err
	}
	if img.Script.DataAt == "rom" {
		gap := alignedGap(img.TextBase, len(img.TextData), img.DataBase)
		if gap > 0 {
			zero := make([]byte, gap)
			if err := emitReadmemhBytes(w, zero); err != nil {
				return err
			}
		}
		if err := emitReadmemhBytes(w, img.DataData); err != nil {
			return err
		}
	}
	return nil
}

func emitReadmemhBytes(w io.Writer, b []byte) error {
	// Pad to a multiple of 4 bytes so we always emit complete words.
	n := (len(b) + 3) &^ 3
	for i := 0; i < n; i += 4 {
		var word [4]byte
		copy(word[:], byteAt(b, i, 4))
		v := binary.LittleEndian.Uint32(word[:])
		if _, err := fmt.Fprintf(w, "%08x\n", v); err != nil {
			return err
		}
	}
	return nil
}

func byteAt(b []byte, i, n int) []byte {
	end := i + n
	if end > len(b) {
		end = len(b)
	}
	if i >= len(b) {
		return nil
	}
	return b[i:end]
}

// ─────────────────────────────────────────────────────────────────────────
// Intel HEX
// ─────────────────────────────────────────────────────────────────────────
//
// Record format:
//
//	:LLAAAATT[DD..]CC
//
// where
//   LL    = data byte count
//   AAAA  = 16-bit address (we use 32-bit via type 04 extended-linear-addr)
//   TT    = record type:
//             00 = data
//             01 = end-of-file
//             04 = extended linear address (sets upper 16 bits of next data)
//   DD..  = LL bytes of data
//   CC    = 2's complement checksum (sum of all bytes mod 256, negated)
//
// We chunk every 16 bytes into a data record. When the absolute address
// crosses a 64 KiB boundary, we emit a type-04 record.
func EmitIntelHex(w io.Writer, img *linker.Image) error {
	if err := emitIntelHexRange(w, img.TextBase, img.TextData); err != nil {
		return err
	}
	if img.Script.DataAt == "rom" {
		// .data follows .text in the same address space, ROM. We write a
		// continuous Intel HEX without bothering to fill the alignment gap
		// (most flashers will leave un-touched ROM at 0xFF anyway).
		if err := emitIntelHexRange(w, img.DataBase, img.DataData); err != nil {
			return err
		}
	} else {
		// .data lives in a different memory; emit it as a separate range
		// in the same hex file. Some flashers won't program two memories
		// from one hex file, but it's a useful debugging artifact.
		if err := emitIntelHexRange(w, img.DataBase, img.DataData); err != nil {
			return err
		}
	}
	// EOF record
	_, err := fmt.Fprintln(w, ":00000001FF")
	return err
}

func emitIntelHexRange(w io.Writer, base uint32, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	addr := base
	cur := uint32(0xFFFFFFFF) // force initial extended-address record
	for off := 0; off < len(data); {
		// Emit type-04 if the upper 16 bits changed.
		hi := uint16(addr >> 16)
		if uint16(cur>>16) != hi {
			if err := writeIntelRecord(w, 0x0000, 0x04, []byte{byte(hi >> 8), byte(hi)}); err != nil {
				return err
			}
			cur = addr
		}
		// chunk size: limited by 16 *and* must not cross a 64 KiB boundary.
		chunk := 16
		if off+chunk > len(data) {
			chunk = len(data) - off
		}
		// boundary clamp
		nextBoundary := (uint32(addr) + 0x10000) &^ 0xFFFF
		if uint32(addr)+uint32(chunk) > nextBoundary {
			chunk = int(nextBoundary - addr)
		}

		if err := writeIntelRecord(w, uint16(addr&0xFFFF), 0x00, data[off:off+chunk]); err != nil {
			return err
		}
		addr += uint32(chunk)
		off += chunk
		cur = addr
	}
	return nil
}

func writeIntelRecord(w io.Writer, addr uint16, typ byte, data []byte) error {
	sum := byte(len(data)) + byte(addr>>8) + byte(addr) + typ
	for _, b := range data {
		sum += b
	}
	cs := byte(0) - sum
	if _, err := fmt.Fprintf(w, ":%02X%04X%02X", len(data), addr, typ); err != nil {
		return err
	}
	for _, b := range data {
		if _, err := fmt.Fprintf(w, "%02X", b); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%02X\n", cs)
	return err
}
