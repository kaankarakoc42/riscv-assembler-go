// Package lexer turns a stream of bytes (one assembly source file) into a
// stream of tokens. Newlines and labels are first-class because RV32I asm is
// line-oriented.
package lexer

import (
	"fmt"
	"unicode"
)

type Kind int

const (
	TokEOF Kind = iota
	TokEOL       // end-of-line (statement separator)

	TokIdent     // mnemonic, register, label name
	TokNumber    // integer literal (possibly negative)
	TokString    // "…", with C-style escapes
	TokDirective // .text, .data, .word, …

	TokComma
	TokLParen
	TokRParen
	TokColon
)

type Token struct {
	Kind Kind
	Text string // raw text or unescaped (for strings)
	Num  int64  // populated for TokNumber

	Line int
	Col  int
}

func (t Token) String() string {
	switch t.Kind {
	case TokEOF:
		return "<EOF>"
	case TokEOL:
		return "<EOL>"
	}
	return fmt.Sprintf("%q", t.Text)
}

// Lex turns src into tokens. filename is purely for error messages.
func Lex(filename string, src []byte) ([]Token, error) {
	l := &lexer{filename: filename, src: src, line: 1, col: 1}
	return l.run()
}

type lexer struct {
	filename string
	src      []byte
	pos      int
	line     int
	col      int
	out      []Token
}

func (l *lexer) errf(format string, a ...any) error {
	return fmt.Errorf("%s:%d:%d: %s", l.filename, l.line, l.col, fmt.Sprintf(format, a...))
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) advance() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

func (l *lexer) emit(k Kind, text string, line, col int) {
	l.out = append(l.out, Token{Kind: k, Text: text, Line: line, Col: col})
}

func (l *lexer) emitNum(n int64, text string, line, col int) {
	l.out = append(l.out, Token{Kind: TokNumber, Text: text, Num: n, Line: line, Col: col})
}

func (l *lexer) run() ([]Token, error) {
	for l.pos < len(l.src) {
		c := l.peek()

		// --- whitespace (but not newline) -------------------------------
		if c == ' ' || c == '\t' || c == '\r' {
			l.advance()
			continue
		}

		// --- newline -----------------------------------------------------
		if c == '\n' {
			line, col := l.line, l.col
			l.advance()
			l.emit(TokEOL, "\n", line, col)
			continue
		}

		// --- comment to end of line --------------------------------------
		if c == '#' || c == ';' {
			for l.pos < len(l.src) && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		if c == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			for l.pos < len(l.src) && l.peek() != '\n' {
				l.advance()
			}
			continue
		}

		line, col := l.line, l.col

		// --- punctuation -------------------------------------------------
		switch c {
		case ',':
			l.advance()
			l.emit(TokComma, ",", line, col)
			continue
		case '(':
			l.advance()
			l.emit(TokLParen, "(", line, col)
			continue
		case ')':
			l.advance()
			l.emit(TokRParen, ")", line, col)
			continue
		case ':':
			l.advance()
			l.emit(TokColon, ":", line, col)
			continue
		}

		// --- string literal ---------------------------------------------
		if c == '"' {
			s, err := l.readString()
			if err != nil {
				return nil, err
			}
			l.emit(TokString, s, line, col)
			continue
		}

		// --- directive (starts with '.') --------------------------------
		if c == '.' {
			l.advance()
			start := l.pos - 1 // include the dot
			for l.pos < len(l.src) && isIdentByte(l.peek()) {
				l.advance()
			}
			l.emit(TokDirective, string(l.src[start:l.pos]), line, col)
			continue
		}

		// --- number ------------------------------------------------------
		if c == '-' || c == '+' || isDigit(c) {
			n, text, err := l.readNumber()
			if err != nil {
				return nil, err
			}
			l.emitNum(n, text, line, col)
			continue
		}

		// --- identifier --------------------------------------------------
		if isIdentStart(c) {
			start := l.pos
			for l.pos < len(l.src) && isIdentByte(l.peek()) {
				l.advance()
			}
			l.emit(TokIdent, string(l.src[start:l.pos]), line, col)
			continue
		}

		return nil, l.errf("unexpected character %q", c)
	}
	l.emit(TokEOF, "", l.line, l.col)
	return l.out, nil
}

func (l *lexer) readString() (string, error) {
	l.advance() // consume opening "
	var buf []byte
	for {
		if l.pos >= len(l.src) {
			return "", l.errf("unterminated string literal")
		}
		c := l.advance()
		if c == '"' {
			return string(buf), nil
		}
		if c == '\\' {
			esc := l.advance()
			switch esc {
			case 'n':
				buf = append(buf, '\n')
			case 'r':
				buf = append(buf, '\r')
			case 't':
				buf = append(buf, '\t')
			case '0':
				buf = append(buf, 0)
			case '\\':
				buf = append(buf, '\\')
			case '"':
				buf = append(buf, '"')
			case '\'':
				buf = append(buf, '\'')
			default:
				return "", l.errf("unknown escape \\%c", esc)
			}
			continue
		}
		buf = append(buf, c)
	}
}

func (l *lexer) readNumber() (int64, string, error) {
	start := l.pos
	neg := false
	if l.peek() == '-' || l.peek() == '+' {
		if l.peek() == '-' {
			neg = true
		}
		l.advance()
	}
	if l.pos >= len(l.src) || !isDigit(l.peek()) {
		return 0, "", l.errf("expected digit")
	}

	var n int64
	base := int64(10)

	if l.peek() == '0' && l.pos+1 < len(l.src) {
		nx := l.src[l.pos+1]
		if nx == 'x' || nx == 'X' {
			base = 16
			l.advance()
			l.advance()
		} else if nx == 'b' || nx == 'B' {
			base = 2
			l.advance()
			l.advance()
		} else if nx == 'o' || nx == 'O' {
			base = 8
			l.advance()
			l.advance()
		}
	}

	any := false
	for l.pos < len(l.src) {
		c := l.peek()
		var d int64
		switch {
		case c >= '0' && c <= '9':
			d = int64(c - '0')
		case c >= 'a' && c <= 'f':
			d = int64(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int64(c-'A') + 10
		case c == '_':
			l.advance()
			continue
		default:
			goto done
		}
		if d >= base {
			goto done
		}
		n = n*base + d
		any = true
		l.advance()
	}
done:
	if !any {
		return 0, "", l.errf("malformed numeric literal")
	}
	if neg {
		n = -n
	}
	return n, string(l.src[start:l.pos]), nil
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool { return c == '_' || unicode.IsLetter(rune(c)) }
func isIdentByte(c byte) bool {
	return c == '_' || c == '.' || isDigit(c) || unicode.IsLetter(rune(c))
}
