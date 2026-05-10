package obj

import (
	"bytes"
	"testing"

	"github.com/edu/rvtoolchain/common/reloc"
)

func TestRoundTripEmpty(t *testing.T) {
	m := &Module{}
	var buf bytes.Buffer
	if err := Write(&buf, m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(buf.Bytes())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got.Sections) != 0 || len(got.Symbols) != 0 || len(got.Relocs) != 0 {
		t.Fatalf("expected empty module, got %+v", got)
	}
}

func TestRoundTripFull(t *testing.T) {
	in := &Module{
		Sections: []Section{
			{Name: ".text", Flags: FlagExec | FlagAlloc, Data: []byte{0x93, 0x00, 0x50, 0x00}},
			{Name: ".data", Flags: FlagWrite | FlagAlloc, Data: []byte("Hello!")},
		},
		Symbols: []Symbol{
			{Name: "main", SectionIdx: 0, Bind: BindGlobal, Value: 0},
			{Name: "msg", SectionIdx: 1, Bind: BindGlobal, Value: 0},
			{Name: "putchar", SectionIdx: SectionExtern, Bind: BindExtern, Value: 0},
		},
		Relocs: []Reloc{
			{SectionIdx: 0, Type: reloc.R_RV32_BRANCH, Offset: 0, SymIdx: 0, Addend: 0},
			{SectionIdx: 0, Type: reloc.R_RV32_HI20, Offset: 4, SymIdx: 1, Addend: 0},
			{SectionIdx: 0, Type: reloc.R_RV32_LO12_I, Offset: 8, SymIdx: 1, Addend: 0},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out, err := Read(buf.Bytes())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(out.Sections) != 2 {
		t.Fatalf("sections = %d", len(out.Sections))
	}
	for i := range in.Sections {
		if in.Sections[i].Name != out.Sections[i].Name {
			t.Errorf("section %d name mismatch", i)
		}
		if in.Sections[i].Flags != out.Sections[i].Flags {
			t.Errorf("section %d flags mismatch: %x vs %x",
				i, in.Sections[i].Flags, out.Sections[i].Flags)
		}
		if !bytes.Equal(in.Sections[i].Data, out.Sections[i].Data) {
			t.Errorf("section %d data mismatch", i)
		}
	}

	if len(out.Symbols) != 3 {
		t.Fatalf("symbols=%d", len(out.Symbols))
	}
	for i := range in.Symbols {
		if in.Symbols[i] != out.Symbols[i] {
			t.Errorf("symbol %d: %+v vs %+v", i, in.Symbols[i], out.Symbols[i])
		}
	}

	if len(out.Relocs) != 3 {
		t.Fatalf("relocs=%d", len(out.Relocs))
	}
	for i := range in.Relocs {
		if in.Relocs[i] != out.Relocs[i] {
			t.Errorf("reloc %d: %+v vs %+v", i, in.Relocs[i], out.Relocs[i])
		}
	}
}

func TestBadMagic(t *testing.T) {
	bad := make([]byte, 24)
	_, err := Read(bad)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}
