package image

import (
	"bytes"
	"strings"
	"testing"

	"github.com/edu/rvtoolchain/linker"
	"github.com/edu/rvtoolchain/linker/script"
)

func mockImage() *linker.Image {
	return &linker.Image{
		Script:   script.Default(),
		TextBase: 0,
		TextData: []byte{0x93, 0x00, 0x50, 0x00, 0x67, 0x80, 0x00, 0x00},
		DataBase: 0x10,
		DataData: []byte{0x41, 0x42, 0x43, 0x44},
	}
}

func TestEmitReadmemh(t *testing.T) {
	img := mockImage()
	var buf bytes.Buffer
	if err := EmitReadmemh(&buf, img); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// .text: two 32-bit words → "00500093\n00008067\n"
	// (data_at = rom default; gap from 0x08 to 0x10 = 8 bytes = 2 zero words)
	// .data: "44434241\n"
	expectLines := []string{
		"00500093",
		"00008067",
		"00000000",
		"00000000",
		"44434241",
	}
	for _, want := range expectLines {
		if !strings.Contains(out, want) {
			t.Errorf(".mem missing line %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestEmitIntelHexValid(t *testing.T) {
	img := mockImage()
	var buf bytes.Buffer
	if err := EmitIntelHex(&buf, img); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	// Must start with a record line beginning with ':' and end with EOF
	// record :00000001FF.
	if !strings.HasPrefix(s, ":") {
		t.Errorf("hex doesn't start with ':' record")
	}
	if !strings.Contains(s, ":00000001FF") {
		t.Errorf("hex missing EOF record")
	}
}

func TestEmitBinSize(t *testing.T) {
	img := mockImage()
	var buf bytes.Buffer
	if err := EmitBin(&buf, img); err != nil {
		t.Fatal(err)
	}
	// text(8) + gap(8) + data(4) = 20
	if buf.Len() != 20 {
		t.Errorf(".bin size = %d, want 20", buf.Len())
	}
	// First word should be the text bytes verbatim
	if !bytes.Equal(buf.Bytes()[:8], img.TextData) {
		t.Errorf(".bin head doesn't match text")
	}
	// Last 4 bytes = data
	if !bytes.Equal(buf.Bytes()[16:20], img.DataData) {
		t.Errorf(".bin tail doesn't match data")
	}
}
