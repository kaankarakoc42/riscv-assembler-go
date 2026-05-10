package obj

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Write marshals m to w in RVOB1 format.
//
// Layout: header (24 B), section descriptors (variable), section payloads
// (raw bytes, in section order), symbol entries, relocation entries.
func Write(w io.Writer, m *Module) error {
	if len(m.Sections) > 255 {
		return fmt.Errorf("obj: too many sections (%d)", len(m.Sections))
	}

	// --- pre-compute payload offsets so descriptors can point to them. -----
	const headerSize = 24
	descSize := 0
	for _, s := range m.Sections {
		descSize += 1 + len(s.Name) + 4 + 4 + 4
	}
	payloadOff := headerSize + descSize
	offsets := make([]uint32, len(m.Sections))
	cursor := uint32(payloadOff)
	for i, s := range m.Sections {
		offsets[i] = cursor
		cursor += uint32(len(s.Data))
	}

	// --- header -----------------------------------------------------------
	var buf [24]byte
	binary.LittleEndian.PutUint32(buf[0:4], Magic)
	binary.LittleEndian.PutUint16(buf[4:6], Version)
	binary.LittleEndian.PutUint16(buf[6:8], 0)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(len(m.Sections)))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(len(m.Symbols)))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(len(m.Relocs)))
	binary.LittleEndian.PutUint32(buf[20:24], uint32(headerSize))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}

	// --- section descriptors ---------------------------------------------
	for i, s := range m.Sections {
		if len(s.Name) > 255 {
			return fmt.Errorf("obj: section name too long: %q", s.Name)
		}
		if _, err := w.Write([]byte{byte(len(s.Name))}); err != nil {
			return err
		}
		if _, err := io.WriteString(w, s.Name); err != nil {
			return err
		}
		var d [12]byte
		binary.LittleEndian.PutUint32(d[0:4], s.Flags)
		binary.LittleEndian.PutUint32(d[4:8], uint32(len(s.Data)))
		binary.LittleEndian.PutUint32(d[8:12], offsets[i])
		if _, err := w.Write(d[:]); err != nil {
			return err
		}
	}

	// --- section payloads -------------------------------------------------
	for _, s := range m.Sections {
		if _, err := w.Write(s.Data); err != nil {
			return err
		}
	}

	// --- symbols ----------------------------------------------------------
	for _, s := range m.Symbols {
		if len(s.Name) > 255 {
			return fmt.Errorf("obj: symbol name too long: %q", s.Name)
		}
		if _, err := w.Write([]byte{byte(len(s.Name))}); err != nil {
			return err
		}
		if _, err := io.WriteString(w, s.Name); err != nil {
			return err
		}
		var d [6]byte
		d[0] = s.SectionIdx
		d[1] = byte(s.Bind)
		binary.LittleEndian.PutUint32(d[2:6], s.Value)
		if _, err := w.Write(d[:]); err != nil {
			return err
		}
	}

	// --- relocations ------------------------------------------------------
	for _, r := range m.Relocs {
		var d [14]byte
		d[0] = r.SectionIdx
		d[1] = byte(r.Type)
		binary.LittleEndian.PutUint32(d[2:6], r.Offset)
		binary.LittleEndian.PutUint32(d[6:10], r.SymIdx)
		binary.LittleEndian.PutUint32(d[10:14], uint32(r.Addend))
		if _, err := w.Write(d[:]); err != nil {
			return err
		}
	}

	return nil
}
