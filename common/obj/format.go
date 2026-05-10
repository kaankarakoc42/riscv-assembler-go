// Package obj defines the in-memory representation of an RVOB1 relocatable
// object and the on-disk reader/writer.
//
// Format spec lives in docs/object-format.md.
package obj

import "github.com/edu/rvtoolchain/common/reloc"

// Magic and version values.
const (
	Magic   uint32 = 0x424F5652 // "RVOB" little-endian
	Version uint16 = 1
)

// SectionFlag bits.
const (
	FlagExec  uint32 = 1 << 0
	FlagWrite uint32 = 1 << 1
	FlagAlloc uint32 = 1 << 2
)

// Symbol bindings.
type Binding uint8

const (
	BindLocal  Binding = 0
	BindGlobal Binding = 1
	BindExtern Binding = 2
)

func (b Binding) String() string {
	switch b {
	case BindLocal:
		return "LOCAL"
	case BindGlobal:
		return "GLOBAL"
	case BindExtern:
		return "EXTERN"
	}
	return "?"
}

// SectionExtern is the marker used in Symbol.SectionIdx for symbols defined
// outside this module.
const SectionExtern = 0xFF

// Section is one contiguous span of bytes (.text, .data, …).
type Section struct {
	Name  string
	Flags uint32
	Data  []byte
}

// Symbol is a named address.
type Symbol struct {
	Name        string
	SectionIdx  uint8 // index into Module.Sections, or SectionExtern
	Bind        Binding
	Value       uint32 // section-relative offset
}

// Reloc is a "fix-me-later" note for the linker.
type Reloc struct {
	SectionIdx uint8
	Type       reloc.Type
	Offset     uint32
	SymIdx     uint32
	Addend     int32
}

// Module is the in-memory shape of one .ro file.
type Module struct {
	Sections []Section
	Symbols  []Symbol
	Relocs   []Reloc
}

// FindSection returns the index of the section with the given name, or -1.
func (m *Module) FindSection(name string) int {
	for i, s := range m.Sections {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// AddOrGetSection returns the index of an existing section by name, creating
// one with the given flags if it doesn't exist yet.
func (m *Module) AddOrGetSection(name string, flags uint32) int {
	if i := m.FindSection(name); i >= 0 {
		return i
	}
	m.Sections = append(m.Sections, Section{Name: name, Flags: flags})
	return len(m.Sections) - 1
}

// AddSymbol appends a symbol and returns its new index.
func (m *Module) AddSymbol(s Symbol) uint32 {
	m.Symbols = append(m.Symbols, s)
	return uint32(len(m.Symbols) - 1)
}
