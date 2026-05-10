package obj

import (
	"encoding/binary"
	"fmt"

	"github.com/edu/rvtoolchain/common/reloc"
)

// Read parses raw RVOB1 bytes into a Module.
//
// We slurp the entire file in advance — object files are small (kilobytes)
// and random-access reading keeps the parser linear and easy to audit.
func Read(data []byte) (*Module, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("obj: file too small (%d bytes)", len(data))
	}
	if magic := binary.LittleEndian.Uint32(data[0:4]); magic != Magic {
		return nil, fmt.Errorf("obj: bad magic 0x%08X (expected 0x%08X)", magic, Magic)
	}
	if v := binary.LittleEndian.Uint16(data[4:6]); v != Version {
		return nil, fmt.Errorf("obj: unsupported version %d", v)
	}
	numSec := binary.LittleEndian.Uint32(data[8:12])
	numSym := binary.LittleEndian.Uint32(data[12:16])
	numRel := binary.LittleEndian.Uint32(data[16:20])
	secOff := binary.LittleEndian.Uint32(data[20:24])

	m := &Module{}
	cur := int(secOff)

	// --- section descriptors ---------------------------------------------
	type pendingSec struct {
		idx     int
		offset  uint32
		size    uint32
	}
	var pendings []pendingSec
	for i := uint32(0); i < numSec; i++ {
		if cur+1 > len(data) {
			return nil, fmt.Errorf("obj: truncated at section %d header", i)
		}
		nameLen := int(data[cur])
		cur++
		if cur+nameLen+12 > len(data) {
			return nil, fmt.Errorf("obj: truncated section %d", i)
		}
		name := string(data[cur : cur+nameLen])
		cur += nameLen
		flags := binary.LittleEndian.Uint32(data[cur : cur+4])
		size := binary.LittleEndian.Uint32(data[cur+4 : cur+8])
		off := binary.LittleEndian.Uint32(data[cur+8 : cur+12])
		cur += 12
		m.Sections = append(m.Sections, Section{Name: name, Flags: flags})
		pendings = append(pendings, pendingSec{idx: int(i), offset: off, size: size})
	}

	// --- section payloads (random access by stored offset) ---------------
	for _, p := range pendings {
		if uint64(p.offset)+uint64(p.size) > uint64(len(data)) {
			return nil, fmt.Errorf("obj: section %d payload OOB", p.idx)
		}
		buf := make([]byte, p.size)
		copy(buf, data[p.offset:p.offset+p.size])
		m.Sections[p.idx].Data = buf
	}

	// After section descriptors and payloads, rewind cursor to after the
	// last payload. We append payloads sequentially, so the symbol table
	// starts right after them.
	last := uint32(secOff) + descSizeOf(m.Sections)
	for _, s := range m.Sections {
		last += uint32(len(s.Data))
	}
	cur = int(last)

	// --- symbols ---------------------------------------------------------
	for i := uint32(0); i < numSym; i++ {
		if cur >= len(data) {
			return nil, fmt.Errorf("obj: truncated at symbol %d", i)
		}
		nameLen := int(data[cur])
		cur++
		if cur+nameLen+6 > len(data) {
			return nil, fmt.Errorf("obj: truncated symbol %d body", i)
		}
		name := string(data[cur : cur+nameLen])
		cur += nameLen
		secIdx := data[cur]
		bind := Binding(data[cur+1])
		val := binary.LittleEndian.Uint32(data[cur+2 : cur+6])
		cur += 6
		m.Symbols = append(m.Symbols, Symbol{
			Name:       name,
			SectionIdx: secIdx,
			Bind:       bind,
			Value:      val,
		})
	}

	// --- relocations -----------------------------------------------------
	for i := uint32(0); i < numRel; i++ {
		if cur+14 > len(data) {
			return nil, fmt.Errorf("obj: truncated reloc %d", i)
		}
		r := Reloc{
			SectionIdx: data[cur],
			Type:       reloc.Type(data[cur+1]),
			Offset:     binary.LittleEndian.Uint32(data[cur+2 : cur+6]),
			SymIdx:     binary.LittleEndian.Uint32(data[cur+6 : cur+10]),
			Addend:     int32(binary.LittleEndian.Uint32(data[cur+10 : cur+14])),
		}
		cur += 14
		m.Relocs = append(m.Relocs, r)
	}

	return m, nil
}

func descSizeOf(secs []Section) uint32 {
	var n uint32
	for _, s := range secs {
		n += 1 + uint32(len(s.Name)) + 4 + 4 + 4
	}
	return n
}
