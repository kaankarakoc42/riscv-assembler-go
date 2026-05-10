package lexer

import "testing"

func collect(t *testing.T, src string) []Token {
	t.Helper()
	toks, err := Lex("test.s", []byte(src))
	if err != nil {
		t.Fatalf("Lex: %v", err)
	}
	return toks
}

func TestSimpleInstruction(t *testing.T) {
	toks := collect(t, "addi x1, x0, 5\n")
	want := []Kind{TokIdent, TokIdent, TokComma, TokIdent, TokComma, TokNumber, TokEOL, TokEOF}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %+v", len(toks), len(want), toks)
	}
	for i, tk := range toks {
		if tk.Kind != want[i] {
			t.Errorf("token %d: kind=%v want %v (%+v)", i, tk.Kind, want[i], tk)
		}
	}
	if toks[5].Num != 5 {
		t.Errorf("number value = %d want 5", toks[5].Num)
	}
}

func TestHexAndBinary(t *testing.T) {
	toks := collect(t, "lui x1, 0xDEADB\n")
	// lui(0) x1(1) ,(2) 0xDEADB(3)
	if toks[3].Num != 0xDEADB {
		t.Fatalf("hex parse failed: %d", toks[3].Num)
	}

	toks2 := collect(t, ".word 0b1010_1010\n")
	if toks2[1].Num != 0xAA {
		t.Fatalf("binary parse with underscores failed: %d", toks2[1].Num)
	}
}

func TestNegativeNumber(t *testing.T) {
	toks := collect(t, "addi x1, x2, -1\n")
	if toks[5].Num != -1 {
		t.Fatalf("negative number = %d, want -1", toks[5].Num)
	}
}

func TestStringLiteral(t *testing.T) {
	toks := collect(t, ".ascii \"hi\\n\"\n")
	if toks[1].Kind != TokString || toks[1].Text != "hi\n" {
		t.Fatalf("string lex = %+v", toks[1])
	}
}

func TestLabelAndComment(t *testing.T) {
	toks := collect(t, "main: # entry point\n  add x1, x2, x3\n")
	if toks[0].Kind != TokIdent || toks[0].Text != "main" {
		t.Fatal("label name not lexed")
	}
	if toks[1].Kind != TokColon {
		t.Fatal("expected colon")
	}
	if toks[2].Kind != TokEOL {
		t.Fatal("comment not stripped to EOL")
	}
}

func TestParenLoadStore(t *testing.T) {
	toks := collect(t, "lw x1, 8(x2)\n")
	kinds := []Kind{TokIdent, TokIdent, TokComma, TokNumber, TokLParen, TokIdent, TokRParen, TokEOL, TokEOF}
	for i, want := range kinds {
		if toks[i].Kind != want {
			t.Errorf("tok %d kind=%v want %v", i, toks[i].Kind, want)
		}
	}
}

func TestDirective(t *testing.T) {
	toks := collect(t, ".global main\n")
	if toks[0].Kind != TokDirective || toks[0].Text != ".global" {
		t.Fatalf("directive lex = %+v", toks[0])
	}
}
