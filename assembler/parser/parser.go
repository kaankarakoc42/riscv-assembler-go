package parser

import (
	"fmt"

	"github.com/edu/rvtoolchain/assembler/lexer"
	"github.com/edu/rvtoolchain/common/isa"
)

// Parse runs the full token stream into a slice of Stmts.
func Parse(filename string, toks []lexer.Token) ([]Stmt, error) {
	p := &parser{filename: filename, toks: toks}
	return p.run()
}

type parser struct {
	filename string
	toks     []lexer.Token
	pos      int
	out      []Stmt
}

func (p *parser) errf(t lexer.Token, format string, a ...any) error {
	return fmt.Errorf("%s:%d:%d: %s", p.filename, t.Line, t.Col, fmt.Sprintf(format, a...))
}

func (p *parser) peek() lexer.Token  { return p.toks[p.pos] }
func (p *parser) at(k lexer.Kind) bool { return p.peek().Kind == k }
func (p *parser) advance() lexer.Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) expect(k lexer.Kind, what string) (lexer.Token, error) {
	t := p.peek()
	if t.Kind != k {
		return t, p.errf(t, "expected %s, got %s", what, t)
	}
	return p.advance(), nil
}

func (p *parser) skipBlankLines() {
	for p.at(lexer.TokEOL) {
		p.advance()
	}
}

func (p *parser) run() ([]Stmt, error) {
	for {
		p.skipBlankLines()
		if p.at(lexer.TokEOF) {
			return p.out, nil
		}
		if err := p.line(); err != nil {
			return nil, err
		}
	}
}

func (p *parser) line() error {
	first := p.peek()

	switch first.Kind {
	case lexer.TokDirective:
		return p.directive()

	case lexer.TokIdent:
		// Either a label (`name:`) or an instruction (`mnemonic ...`).
		if p.pos+1 < len(p.toks) && p.toks[p.pos+1].Kind == lexer.TokColon {
			name := p.advance().Text
			p.advance() // consume ':'
			p.out = append(p.out, Stmt{Kind: StmtLabel, Label: name, File: p.filename, Line: first.Line})
			// Allow optional more on same line: "main: addi x1,x0,1"
			if p.at(lexer.TokEOL) || p.at(lexer.TokEOF) {
				if p.at(lexer.TokEOL) {
					p.advance()
				}
				return nil
			}
			return p.line()
		}
		return p.instruction()

	default:
		return p.errf(first, "unexpected token at start of line: %s", first)
	}
}

func (p *parser) directive() error {
	d := p.advance()
	name := d.Text[1:] // strip leading '.'

	stmt := Stmt{Kind: StmtDirective, Directive: name, File: p.filename, Line: d.Line}

	for !p.at(lexer.TokEOL) && !p.at(lexer.TokEOF) {
		switch p.peek().Kind {
		case lexer.TokString:
			stmt.Strings = append(stmt.Strings, p.advance().Text)
		case lexer.TokIdent:
			stmt.Operands = append(stmt.Operands, Operand{Kind: OpSym, Sym: p.advance().Text})
		case lexer.TokNumber:
			stmt.Operands = append(stmt.Operands, Operand{Kind: OpNum, Num: p.advance().Num})
		case lexer.TokComma:
			p.advance() // ignore commas — they're optional separators
		default:
			return p.errf(p.peek(), "unexpected token in directive: %s", p.peek())
		}
	}
	if p.at(lexer.TokEOL) {
		p.advance()
	}
	p.out = append(p.out, stmt)
	return nil
}

func (p *parser) instruction() error {
	mn := p.advance() // mnemonic
	stmt := Stmt{
		Kind:     StmtInstr,
		Mnemonic: mn.Text,
		File:     p.filename,
		Line:     mn.Line,
	}

	// Operand list, comma-separated. Stop on EOL/EOF.
	for {
		if p.at(lexer.TokEOL) || p.at(lexer.TokEOF) {
			break
		}
		if len(stmt.Operands) > 0 {
			if _, err := p.expect(lexer.TokComma, "','"); err != nil {
				return err
			}
		}
		ops, err := p.parseOperandGroup()
		if err != nil {
			return err
		}
		stmt.Operands = append(stmt.Operands, ops...)
	}
	if p.at(lexer.TokEOL) {
		p.advance()
	}
	p.out = append(p.out, stmt)
	return nil
}

// parseOperandGroup parses one or two operands from a single comma-separated
// "slot". Most slots produce a single operand. The exception is the
// memory-addressing form `imm(reg)` (used by lw/sw/lb/etc.), which produces
// two operands in this order: immediate, then base register.
//
// Supported slot shapes:
//
//	x5                        → [reg]
//	-12                       → [num]
//	0(x5)                     → [num, reg]
//	%lo(sym)(x5)              → [sym%lo, reg]
//	main                      → [sym]
//	%hi(main) / %lo(main)     → [sym with modifier]
//	%pcrel_hi(main)
//	%pcrel_lo(label)
func (p *parser) parseOperandGroup() ([]Operand, error) {
	head, err := p.parseOperandAtom()
	if err != nil {
		return nil, err
	}

	// `imm(reg)` form. Trigger only if the head is a number or a
	// symbol-with-modifier and the next token is '('.
	if p.at(lexer.TokLParen) {
		canCarryParen := head.Kind == OpNum || (head.Kind == OpSym && head.Mod != ModNone)
		if !canCarryParen {
			return nil, p.errf(p.peek(), "unexpected '(' after operand")
		}
		p.advance() // '('
		if !p.at(lexer.TokIdent) {
			return nil, p.errf(p.peek(), "expected register inside parentheses")
		}
		t := p.advance()
		r, ok := isa.Reg(t.Text)
		if !ok {
			return nil, p.errf(t, "%q is not a register", t.Text)
		}
		if _, err := p.expect(lexer.TokRParen, "')'"); err != nil {
			return nil, err
		}
		return []Operand{head, {Kind: OpReg, Reg: r}}, nil
	}

	return []Operand{head}, nil
}

// parseOperandAtom parses a single operand, NOT including the trailing
// `(reg)` in memory-addressing forms.
func (p *parser) parseOperandAtom() (Operand, error) {
	t := p.peek()

	switch t.Kind {
	case lexer.TokNumber:
		p.advance()
		return Operand{Kind: OpNum, Num: t.Num}, nil

	case lexer.TokIdent:
		// Modifiers: hi, lo, pcrel_hi, pcrel_lo — followed by "(sym)".
		if mod := identToMod(t.Text); mod != ModNone {
			if p.pos+1 < len(p.toks) && p.toks[p.pos+1].Kind == lexer.TokLParen {
				p.advance() // modifier name
				p.advance() // '('
				if !p.at(lexer.TokIdent) {
					return Operand{}, p.errf(p.peek(), "expected symbol inside %s(...)", t.Text)
				}
				sym := p.advance().Text
				if _, err := p.expect(lexer.TokRParen, "')'"); err != nil {
					return Operand{}, err
				}
				return Operand{Kind: OpSym, Sym: sym, Mod: mod}, nil
			}
		}
		// Otherwise: register or plain symbol.
		if r, ok := isa.Reg(t.Text); ok {
			p.advance()
			return Operand{Kind: OpReg, Reg: r}, nil
		}
		p.advance()
		return Operand{Kind: OpSym, Sym: t.Text}, nil
	}

	return Operand{}, p.errf(t, "expected operand, got %s", t)
}

func identToMod(s string) Modifier {
	switch s {
	case "hi":
		return ModHI
	case "lo":
		return ModLO
	case "pcrel_hi":
		return ModPCHI
	case "pcrel_lo":
		return ModPCLO
	}
	return ModNone
}
