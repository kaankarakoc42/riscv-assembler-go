// Package script parses the linker script.
//
// The script is a deliberately tiny INI-ish format: line-based, key=value,
// with [section]-style headers. We don't import an INI/TOML library — the
// implementation is a few dozen lines.
//
// Example:
//
//	# blink.lds — PicoRV32 default
//	[memory]
//	rom_base = 0x00000000
//	rom_size = 0x2000        # 8 KiB BRAM for instruction memory
//	ram_base = 0x00010000    # could be a separate BRAM, or just .data after .text
//	ram_size = 0x1000
//
//	[layout]
//	text_at = rom
//	data_at = rom            # follow .text in BRAM
//	entry   = 0x00000000     # PicoRV32 reset vector
//
// Notes:
//   - "text_at = rom" means: place .text at rom_base.
//   - "data_at = rom" means: place .data immediately after .text in rom.
//   - "data_at = ram" means: place .data at ram_base.
//   - The "entry" address is informational; PicoRV32 always starts at the
//     ROM base regardless.
package script

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Script is the parsed result.
type Script struct {
	RomBase uint32
	RomSize uint32
	RamBase uint32
	RamSize uint32

	TextAt string // "rom" or "ram"
	DataAt string // "rom" or "ram"
	Entry  uint32
}

// Default returns a sensible default for PicoRV32 demos — ROM at 0, .text and
// .data both there, in 8 KiB.
func Default() *Script {
	return &Script{
		RomBase: 0x00000000,
		RomSize: 0x2000,
		RamBase: 0x00010000,
		RamSize: 0x1000,
		TextAt:  "rom",
		DataAt:  "rom",
		Entry:   0,
	}
}

// Parse reads a script from raw bytes.
func Parse(src []byte) (*Script, error) {
	s := Default()

	sc := bufio.NewScanner(bytes.NewReader(src))
	section := ""
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		// strip trailing inline comment
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil, fmt.Errorf("script:%d: missing '=' in %q", lineNo, line)
		}
		key := strings.TrimSpace(strings.ToLower(line[:eq]))
		val := strings.TrimSpace(line[eq+1:])

		switch section {
		case "memory":
			n, err := parseUint(val)
			if err != nil {
				return nil, fmt.Errorf("script:%d: %v", lineNo, err)
			}
			switch key {
			case "rom_base":
				s.RomBase = n
			case "rom_size":
				s.RomSize = n
			case "ram_base":
				s.RamBase = n
			case "ram_size":
				s.RamSize = n
			default:
				return nil, fmt.Errorf("script:%d: unknown memory key %q", lineNo, key)
			}
		case "layout":
			switch key {
			case "text_at":
				s.TextAt = strings.ToLower(val)
			case "data_at":
				s.DataAt = strings.ToLower(val)
			case "entry":
				n, err := parseUint(val)
				if err != nil {
					return nil, fmt.Errorf("script:%d: %v", lineNo, err)
				}
				s.Entry = n
			default:
				return nil, fmt.Errorf("script:%d: unknown layout key %q", lineNo, key)
			}
		default:
			return nil, fmt.Errorf("script:%d: key outside any [section]", lineNo)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if s.TextAt != "rom" && s.TextAt != "ram" {
		return nil, fmt.Errorf("text_at must be \"rom\" or \"ram\"")
	}
	if s.DataAt != "rom" && s.DataAt != "ram" {
		return nil, fmt.Errorf("data_at must be \"rom\" or \"ram\"")
	}
	return s, nil
}

func parseUint(s string) (uint32, error) {
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		s = s[2:]
	} else if strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B") {
		base = 2
		s = s[2:]
	}
	s = strings.ReplaceAll(s, "_", "")
	n, err := strconv.ParseUint(s, base, 32)
	if err != nil {
		return 0, fmt.Errorf("bad integer %q: %v", s, err)
	}
	return uint32(n), nil
}
