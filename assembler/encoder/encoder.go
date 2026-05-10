// Package encoder turns the parsed AST into a finalised obj.Module ready to
// be written as RVOB1.
//
// Strategy: two passes.
//
//   Pass 1 — measure & label.
//     Walk every statement, decide which section it belongs to (.text or .data),
//     advance a per-section cursor, and record every label as a local symbol
//     at that cursor. Also record .global / .extern declarations.
//
//   Pass 2 — emit.
//     Walk the AST again; for each instruction, encode bytes, and for each
//     symbolic operand emit a relocation referencing the section/offset.
//     Pseudo-instructions are expanded here.
//
// Why two passes? Forward references. `beq x1,x2,target` may appear before
// `target:`. We must know the section sizes (and therefore label offsets)
// before we can emit the relocation site.
package encoder

import (
	"encoding/binary"
	"fmt"

	"github.com/edu/rvtoolchain/assembler/parser"
	"github.com/edu/rvtoolchain/common/isa"
	"github.com/edu/rvtoolchain/common/obj"
	"github.com/edu/rvtoolchain/common/reloc"
)

// Encode runs both passes over stmts and returns a populated module.
func Encode(stmts []parser.Stmt) (*obj.Module, error) {
	e := &encoder{m: &obj.Module{}, symIdx: map[string]uint32{}}
	// Always create .text and .data up front, even if empty. Linker doesn't
	// care; this just makes section indexing predictable across modules.
	e.textIdx = e.m.AddOrGetSection(".text", obj.FlagExec|obj.FlagAlloc)
	e.dataIdx = e.m.AddOrGetSection(".data", obj.FlagWrite|obj.FlagAlloc)

	if err := e.pass1(stmts); err != nil {
		return nil, err
	}
	if err := e.pass2(stmts); err != nil {
		return nil, err
	}
	return e.m, nil
}

type encoder struct {
	m       *obj.Module
	textIdx int
	dataIdx int

	// pass1 outputs:
	textSize uint32
	dataSize uint32

	// label name → symbol index (filled in pass1).
	symIdx map[string]uint32

	// names declared `.global` (kept until pass2 finalises symbol bindings).
	globals map[string]struct{}
	externs map[string]struct{}
}

// ─────────────────────────────────────────────────────────────────────────────
// PASS 1: measure section sizes & assign label offsets
// ─────────────────────────────────────────────────────────────────────────────

func (e *encoder) pass1(stmts []parser.Stmt) error {
	e.globals = map[string]struct{}{}
	e.externs = map[string]struct{}{}

	curSection := e.textIdx
	cur := &e.textSize

	for _, s := range stmts {
		switch s.Kind {
		case parser.StmtDirective:
			switch s.Directive {
			case "text":
				curSection, cur = e.textIdx, &e.textSize
			case "data":
				curSection, cur = e.dataIdx, &e.dataSize
			case "global":
				for _, op := range s.Operands {
					if op.Kind == parser.OpSym {
						e.globals[op.Sym] = struct{}{}
					}
				}
			case "extern":
				for _, op := range s.Operands {
					if op.Kind == parser.OpSym {
						e.externs[op.Sym] = struct{}{}
					}
				}
			case "word":
				*cur += 4 * uint32(len(s.Operands))
			case "byte":
				*cur += uint32(len(s.Operands))
			case "half":
				*cur += 2 * uint32(len(s.Operands))
			case "ascii":
				for _, str := range s.Strings {
					*cur += uint32(len(str))
				}
			case "asciz", "string":
				for _, str := range s.Strings {
					*cur += uint32(len(str)) + 1
				}
			case "align":
				if len(s.Operands) != 1 || s.Operands[0].Kind != parser.OpNum {
					return fmt.Errorf("%s:%d: .align expects integer power-of-two byte count", s.File, s.Line)
				}
				al := uint32(s.Operands[0].Num)
				if al == 0 || al&(al-1) != 0 {
					return fmt.Errorf("%s:%d: .align value must be a power of two", s.File, s.Line)
				}
				if rem := *cur & (al - 1); rem != 0 {
					*cur += al - rem
				}
			case "space", "skip":
				if len(s.Operands) != 1 || s.Operands[0].Kind != parser.OpNum {
					return fmt.Errorf("%s:%d: .%s expects byte count", s.File, s.Line, s.Directive)
				}
				*cur += uint32(s.Operands[0].Num)
			default:
				return fmt.Errorf("%s:%d: unknown directive .%s", s.File, s.Line, s.Directive)
			}

		case parser.StmtLabel:
			if _, dup := e.symIdx[s.Label]; dup {
				return fmt.Errorf("%s:%d: duplicate label %q", s.File, s.Line, s.Label)
			}
			idx := e.m.AddSymbol(obj.Symbol{
				Name:       s.Label,
				SectionIdx: uint8(curSection),
				Bind:       obj.BindLocal, // upgraded to GLOBAL in pass2 if .global'd
				Value:      *cur,
			})
			e.symIdx[s.Label] = idx

		case parser.StmtInstr:
			n, err := pseudoSize(s.Mnemonic)
			if err != nil {
				return fmt.Errorf("%s:%d: %v", s.File, s.Line, err)
			}
			if curSection != e.textIdx {
				return fmt.Errorf("%s:%d: instruction outside .text section", s.File, s.Line)
			}
			*cur += uint32(n) * 4
		}
	}

	// Allocate the section payload buffers now that we know their sizes.
	e.m.Sections[e.textIdx].Data = make([]byte, e.textSize)
	e.m.Sections[e.dataIdx].Data = make([]byte, e.dataSize)

	// Apply .global / .extern.
	for name := range e.globals {
		if idx, ok := e.symIdx[name]; ok {
			e.m.Symbols[idx].Bind = obj.BindGlobal
		} else {
			// Declared global but not (yet) defined locally. We treat this
			// as "reserved global"; if it's never defined we'd need an
			// extern entry — but `.global foo` typically pairs with a local
			// `foo:`. Falls through to extern handling for safety.
		}
	}
	for name := range e.externs {
		if _, ok := e.symIdx[name]; ok {
			return fmt.Errorf("symbol %q declared .extern but also defined locally", name)
		}
		idx := e.m.AddSymbol(obj.Symbol{
			Name:       name,
			SectionIdx: obj.SectionExtern,
			Bind:       obj.BindExtern,
		})
		e.symIdx[name] = idx
	}

	return nil
}

// pseudoSize returns the number of *real* RV32I instructions a mnemonic
// expands to. Used in pass1 to compute section size.
func pseudoSize(mn string) (int, error) {
	if _, ok := isa.LookupOp(mn); ok {
		return 1, nil
	}
	switch mn {
	case "nop", "mv", "neg", "not", "ret", "j", "jr", "li_small", "bnez", "beqz", "bgez", "bltz", "blez", "bgtz":
		return 1, nil
	case "li", "la", "call", "tail":
		return 2, nil
	}
	return 0, fmt.Errorf("unknown mnemonic %q", mn)
}

// ─────────────────────────────────────────────────────────────────────────────
// PASS 2: emit bytes & relocations
// ─────────────────────────────────────────────────────────────────────────────

func (e *encoder) pass2(stmts []parser.Stmt) error {
	curSection := e.textIdx
	curOff := func() *uint32 {
		if curSection == e.textIdx {
			return new(uint32) // shadowed below
		}
		return new(uint32)
	}
	_ = curOff

	textPos := uint32(0)
	dataPos := uint32(0)

	for _, s := range stmts {
		switch s.Kind {
		case parser.StmtDirective:
			switch s.Directive {
			case "text":
				curSection = e.textIdx
			case "data":
				curSection = e.dataIdx
			case "global", "extern":
				// already handled in pass1

			case "word":
				for _, op := range s.Operands {
					switch op.Kind {
					case parser.OpNum:
						e.writeWord(curSection, e.cursor(curSection, &textPos, &dataPos), uint32(op.Num))
					case parser.OpSym:
						symIdx, err := e.lookupSym(op.Sym, s)
						if err != nil {
							return err
						}
						off := e.cursor(curSection, &textPos, &dataPos)
						e.m.Relocs = append(e.m.Relocs, obj.Reloc{
							SectionIdx: uint8(curSection),
							Type:       reloc.R_RV32_32,
							Offset:     off,
							SymIdx:     symIdx,
						})
						// leave bytes 0 — linker fills them in
					default:
						return fmt.Errorf("%s:%d: bad .word operand", s.File, s.Line)
					}
					e.advance(curSection, &textPos, &dataPos, 4)
				}

			case "half":
				for _, op := range s.Operands {
					if op.Kind != parser.OpNum {
						return fmt.Errorf("%s:%d: .half expects number", s.File, s.Line)
					}
					off := e.cursor(curSection, &textPos, &dataPos)
					binary.LittleEndian.PutUint16(e.m.Sections[curSection].Data[off:], uint16(op.Num))
					e.advance(curSection, &textPos, &dataPos, 2)
				}

			case "byte":
				for _, op := range s.Operands {
					if op.Kind != parser.OpNum {
						return fmt.Errorf("%s:%d: .byte expects number", s.File, s.Line)
					}
					off := e.cursor(curSection, &textPos, &dataPos)
					e.m.Sections[curSection].Data[off] = byte(op.Num)
					e.advance(curSection, &textPos, &dataPos, 1)
				}

			case "ascii":
				for _, str := range s.Strings {
					off := e.cursor(curSection, &textPos, &dataPos)
					copy(e.m.Sections[curSection].Data[off:], []byte(str))
					e.advance(curSection, &textPos, &dataPos, uint32(len(str)))
				}

			case "asciz", "string":
				for _, str := range s.Strings {
					off := e.cursor(curSection, &textPos, &dataPos)
					copy(e.m.Sections[curSection].Data[off:], []byte(str))
					e.advance(curSection, &textPos, &dataPos, uint32(len(str)))
					// trailing NUL
					off2 := e.cursor(curSection, &textPos, &dataPos)
					e.m.Sections[curSection].Data[off2] = 0
					e.advance(curSection, &textPos, &dataPos, 1)
				}

			case "align":
				al := uint32(s.Operands[0].Num)
				cur := e.cursor(curSection, &textPos, &dataPos)
				if rem := cur & (al - 1); rem != 0 {
					e.advance(curSection, &textPos, &dataPos, al-rem)
				}

			case "space", "skip":
				e.advance(curSection, &textPos, &dataPos, uint32(s.Operands[0].Num))
			}

		case parser.StmtLabel:
			// Already handled in pass1.

		case parser.StmtInstr:
			pc := textPos
			words, err := e.encodeInstr(pc, &s)
			if err != nil {
				return fmt.Errorf("%s:%d: %v", s.File, s.Line, err)
			}
			for _, w := range words {
				binary.LittleEndian.PutUint32(e.m.Sections[e.textIdx].Data[textPos:], w)
				textPos += 4
			}
		}
	}
	return nil
}

func (e *encoder) cursor(sec int, t, d *uint32) uint32 {
	if sec == e.textIdx {
		return *t
	}
	return *d
}

func (e *encoder) advance(sec int, t, d *uint32, n uint32) {
	if sec == e.textIdx {
		*t += n
	} else {
		*d += n
	}
}

func (e *encoder) writeWord(sec int, off, val uint32) {
	binary.LittleEndian.PutUint32(e.m.Sections[sec].Data[off:], val)
}

func (e *encoder) lookupSym(name string, s parser.Stmt) (uint32, error) {
	if idx, ok := e.symIdx[name]; ok {
		return idx, nil
	}
	// Auto-extern: an unseen symbol referenced from code is treated as
	// external. The linker will resolve or error.
	idx := e.m.AddSymbol(obj.Symbol{
		Name:       name,
		SectionIdx: obj.SectionExtern,
		Bind:       obj.BindExtern,
	})
	e.symIdx[name] = idx
	return idx, nil
}
