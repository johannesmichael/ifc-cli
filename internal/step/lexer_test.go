package step

import (
	"testing"
)

func TestStructuralTokens(t *testing.T) {
	src := []byte("(),;=")
	lex := NewLexer(src)

	expected := []struct {
		kind  TokenKind
		value string
	}{
		{TokenLParen, "("},
		{TokenRParen, ")"},
		{TokenComma, ","},
		{TokenSemicolon, ";"},
		{TokenEquals, "="},
		{TokenEOF, ""},
	}

	for _, exp := range expected {
		tok, err := lex.NextToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.Kind != exp.kind {
			t.Errorf("expected kind %v, got %v", exp.kind, tok.Kind)
		}
		if tok.Value != exp.value {
			t.Errorf("expected value %q, got %q", exp.value, tok.Value)
		}
	}
}

func TestNullAndDerived(t *testing.T) {
	src := []byte("$ *")
	lex := NewLexer(src)

	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenNull || tok.Value != "$" {
		t.Errorf("expected Null/$, got %v/%q", tok.Kind, tok.Value)
	}

	tok, err = lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenDerived || tok.Value != "*" {
		t.Errorf("expected Derived/*, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestWhitespaceSkipping(t *testing.T) {
	src := []byte("  \t\n\r  ( \n )")
	lex := NewLexer(src)

	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenLParen {
		t.Errorf("expected LParen, got %v", tok.Kind)
	}

	tok, err = lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenRParen {
		t.Errorf("expected RParen, got %v", tok.Kind)
	}
}

func TestCommentSkipping(t *testing.T) {
	src := []byte("( /* this is a comment */ )")
	lex := NewLexer(src)

	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenLParen {
		t.Errorf("expected LParen, got %v", tok.Kind)
	}

	tok, err = lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenRParen {
		t.Errorf("expected RParen, got %v", tok.Kind)
	}
}

func TestUnterminatedComment(t *testing.T) {
	src := []byte("( /* unterminated")
	lex := NewLexer(src)

	_, _ = lex.NextToken() // LParen
	_, err := lex.NextToken()
	if err == nil {
		t.Fatal("expected error for unterminated comment")
	}
}

func TestEOF(t *testing.T) {
	src := []byte("")
	lex := NewLexer(src)

	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenEOF {
		t.Errorf("expected EOF, got %v", tok.Kind)
	}

	// Repeated EOF calls should be safe
	tok, err = lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenEOF {
		t.Errorf("expected EOF on second call, got %v", tok.Kind)
	}
}

func TestUnexpectedCharacter(t *testing.T) {
	src := []byte("@")
	lex := NewLexer(src)

	_, err := lex.NextToken()
	if err == nil {
		t.Fatal("expected error for unexpected character")
	}
}

func TestTokenPositions(t *testing.T) {
	src := []byte("  (  )")
	lex := NewLexer(src)

	tok, _ := lex.NextToken()
	if tok.Pos != 2 {
		t.Errorf("expected pos 2, got %d", tok.Pos)
	}

	tok, _ = lex.NextToken()
	if tok.Pos != 5 {
		t.Errorf("expected pos 5, got %d", tok.Pos)
	}
}

func TestTokenKindString(t *testing.T) {
	if TokenLParen.String() != "LParen" {
		t.Errorf("expected \"LParen\", got %q", TokenLParen.String())
	}
	if TokenEOF.String() != "EOF" {
		t.Errorf("expected \"EOF\", got %q", TokenEOF.String())
	}
}
