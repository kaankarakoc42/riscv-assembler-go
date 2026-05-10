package script

import "testing"

func TestParseFull(t *testing.T) {
	src := `
# blink.lds — PicoRV32 default
[memory]
rom_base = 0x00000000
rom_size = 0x2000
ram_base = 0x10000000
ram_size = 0x800

[layout]
text_at = rom
data_at = rom
entry   = 0x0
`
	s, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.RomBase != 0 || s.RomSize != 0x2000 {
		t.Errorf("rom: %+v", s)
	}
	if s.RamBase != 0x10000000 || s.RamSize != 0x800 {
		t.Errorf("ram: %+v", s)
	}
	if s.TextAt != "rom" || s.DataAt != "rom" {
		t.Errorf("layout: %+v", s)
	}
}

func TestParseDefaults(t *testing.T) {
	s, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("empty parse: %v", err)
	}
	if s.RomSize == 0 {
		t.Errorf("empty script should still have defaults: %+v", s)
	}
}

func TestParseBadLayout(t *testing.T) {
	src := `[layout]
text_at = bogus
`
	if _, err := Parse([]byte(src)); err == nil {
		t.Fatal("expected error for bogus text_at")
	}
}
