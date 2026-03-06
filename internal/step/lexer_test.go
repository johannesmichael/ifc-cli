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

// --- Task 1.3: Numeric literals ---

func TestIntegerPositive(t *testing.T) {
	lex := NewLexer([]byte("42"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenInteger || tok.Value != "42" {
		t.Errorf("expected Integer/42, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestIntegerNegative(t *testing.T) {
	lex := NewLexer([]byte("-7"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenInteger || tok.Value != "-7" {
		t.Errorf("expected Integer/-7, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestIntegerZero(t *testing.T) {
	lex := NewLexer([]byte("0"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenInteger || tok.Value != "0" {
		t.Errorf("expected Integer/0, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestFloatWithDecimal(t *testing.T) {
	lex := NewLexer([]byte("3.14"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenFloat || tok.Value != "3.14" {
		t.Errorf("expected Float/3.14, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestFloatWithExponent(t *testing.T) {
	lex := NewLexer([]byte("5E10"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenFloat || tok.Value != "5E10" {
		t.Errorf("expected Float/5E10, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestFloatWithDecimalAndExponent(t *testing.T) {
	lex := NewLexer([]byte("1.5E-3"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenFloat || tok.Value != "1.5E-3" {
		t.Errorf("expected Float/1.5E-3, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestFloatNegative(t *testing.T) {
	lex := NewLexer([]byte("-0.5"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenFloat || tok.Value != "-0.5" {
		t.Errorf("expected Float/-0.5, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestFloatScientificNegative(t *testing.T) {
	lex := NewLexer([]byte("-2.5e+4"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenFloat || tok.Value != "-2.5e+4" {
		t.Errorf("expected Float/-2.5e+4, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestFloatTrailingDecimal(t *testing.T) {
	lex := NewLexer([]byte("10."))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenFloat || tok.Value != "10." {
		t.Errorf("expected Float/10., got %v/%q", tok.Kind, tok.Value)
	}
}

// --- Task 1.4: String literals ---

func TestStringSimple(t *testing.T) {
	lex := NewLexer([]byte("'Hello world'"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenString || tok.Value != "Hello world" {
		t.Errorf("expected String/Hello world, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestStringEmpty(t *testing.T) {
	lex := NewLexer([]byte("''"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenString || tok.Value != "" {
		t.Errorf("expected String/(empty), got %v/%q", tok.Kind, tok.Value)
	}
}

func TestStringEscapedQuotes(t *testing.T) {
	lex := NewLexer([]byte("'it''s a test'"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenString || tok.Value != "it's a test" {
		t.Errorf("expected String/it's a test, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestStringMultiLine(t *testing.T) {
	lex := NewLexer([]byte("'line1\nline2'"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenString || tok.Value != "line1\nline2" {
		t.Errorf("expected multi-line string, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestStringUnterminated(t *testing.T) {
	lex := NewLexer([]byte("'unterminated"))
	_, err := lex.NextToken()
	if err == nil {
		t.Fatal("expected error for unterminated string")
	}
}

// --- Task 1.6: Entity refs ---

func TestEntityRef(t *testing.T) {
	lex := NewLexer([]byte("#1"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenRef || tok.Value != "#1" {
		t.Errorf("expected Ref/#1, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestEntityRefLarge(t *testing.T) {
	lex := NewLexer([]byte("#12345"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenRef || tok.Value != "#12345" {
		t.Errorf("expected Ref/#12345, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestEntityRefInvalid(t *testing.T) {
	lex := NewLexer([]byte("#abc"))
	_, err := lex.NextToken()
	if err == nil {
		t.Fatal("expected error for # without digits")
	}
}

// --- Task 1.6: Type names ---

func TestTypeName(t *testing.T) {
	lex := NewLexer([]byte("IFCWALL"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenTypeName || tok.Value != "IFCWALL" {
		t.Errorf("expected TypeName/IFCWALL, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestTypeNameComplex(t *testing.T) {
	lex := NewLexer([]byte("IFCPROPERTYSINGLEVALUE"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenTypeName || tok.Value != "IFCPROPERTYSINGLEVALUE" {
		t.Errorf("expected TypeName/IFCPROPERTYSINGLEVALUE, got %v/%q", tok.Kind, tok.Value)
	}
}

func TestTypeNameWithUnderscore(t *testing.T) {
	lex := NewLexer([]byte("ISO_10303_21"))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenTypeName || tok.Value != "ISO_10303_21" {
		t.Errorf("expected TypeName/ISO_10303_21, got %v/%q", tok.Kind, tok.Value)
	}
}

// --- Task 1.7: Header keywords ---

func TestHeaderKeywords(t *testing.T) {
	keywords := []string{"ISO-10303-21", "HEADER", "ENDSEC", "DATA", "END-ISO-10303-21"}
	for _, kw := range keywords {
		lex := NewLexer([]byte(kw))
		tok, err := lex.NextToken()
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", kw, err)
		}
		if tok.Kind != TokenTypeName || tok.Value != kw {
			t.Errorf("expected TypeName/%s, got %v/%q", kw, tok.Kind, tok.Value)
		}
	}
}

// --- Task 1.6: Enums ---

func TestEnum(t *testing.T) {
	lex := NewLexer([]byte(".ELEMENT."))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenEnum || tok.Value != ".ELEMENT." {
		t.Errorf("expected Enum/.ELEMENT., got %v/%q", tok.Kind, tok.Value)
	}
}

func TestEnumTrue(t *testing.T) {
	lex := NewLexer([]byte(".T."))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenEnum || tok.Value != ".T." {
		t.Errorf("expected Enum/.T., got %v/%q", tok.Kind, tok.Value)
	}
}

func TestEnumFalse(t *testing.T) {
	lex := NewLexer([]byte(".F."))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenEnum || tok.Value != ".F." {
		t.Errorf("expected Enum/.F., got %v/%q", tok.Kind, tok.Value)
	}
}

func TestEnumNotDefined(t *testing.T) {
	lex := NewLexer([]byte(".NOTDEFINED."))
	tok, err := lex.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.Kind != TokenEnum || tok.Value != ".NOTDEFINED." {
		t.Errorf("expected Enum/.NOTDEFINED., got %v/%q", tok.Kind, tok.Value)
	}
}

// --- Combined test: realistic STEP fragment ---

func TestSTEPFragment(t *testing.T) {
	src := []byte("#1 = IFCWALL('name',.ELEMENT.,#2,(#3,#4),$);")
	lex := NewLexer(src)

	expected := []struct {
		kind  TokenKind
		value string
	}{
		{TokenRef, "#1"},
		{TokenEquals, "="},
		{TokenTypeName, "IFCWALL"},
		{TokenLParen, "("},
		{TokenString, "name"},
		{TokenComma, ","},
		{TokenEnum, ".ELEMENT."},
		{TokenComma, ","},
		{TokenRef, "#2"},
		{TokenComma, ","},
		{TokenLParen, "("},
		{TokenRef, "#3"},
		{TokenComma, ","},
		{TokenRef, "#4"},
		{TokenRParen, ")"},
		{TokenComma, ","},
		{TokenNull, "$"},
		{TokenRParen, ")"},
		{TokenSemicolon, ";"},
		{TokenEOF, ""},
	}

	for i, exp := range expected {
		tok, err := lex.NextToken()
		if err != nil {
			t.Fatalf("token %d: unexpected error: %v", i, err)
		}
		if tok.Kind != exp.kind {
			t.Errorf("token %d: expected kind %v, got %v", i, exp.kind, tok.Kind)
		}
		if tok.Value != exp.value {
			t.Errorf("token %d: expected value %q, got %q", i, exp.value, tok.Value)
		}
	}
}

func TestSTEPFragmentWithNumbers(t *testing.T) {
	src := []byte("#10 = IFCLENGTHMEASURE(3.14,-7,1.5E-3);")
	lex := NewLexer(src)

	expected := []struct {
		kind  TokenKind
		value string
	}{
		{TokenRef, "#10"},
		{TokenEquals, "="},
		{TokenTypeName, "IFCLENGTHMEASURE"},
		{TokenLParen, "("},
		{TokenFloat, "3.14"},
		{TokenComma, ","},
		{TokenInteger, "-7"},
		{TokenComma, ","},
		{TokenFloat, "1.5E-3"},
		{TokenRParen, ")"},
		{TokenSemicolon, ";"},
		{TokenEOF, ""},
	}

	for i, exp := range expected {
		tok, err := lex.NextToken()
		if err != nil {
			t.Fatalf("token %d: unexpected error: %v", i, err)
		}
		if tok.Kind != exp.kind {
			t.Errorf("token %d: expected kind %v, got %v", i, exp.kind, tok.Kind)
		}
		if tok.Value != exp.value {
			t.Errorf("token %d: expected value %q, got %q", i, exp.value, tok.Value)
		}
	}
}

func TestHeaderSection(t *testing.T) {
	src := []byte("ISO-10303-21;\nHEADER;\nENDSEC;\nDATA;\nENDSEC;\nEND-ISO-10303-21;")
	lex := NewLexer(src)

	expected := []struct {
		kind  TokenKind
		value string
	}{
		{TokenTypeName, "ISO-10303-21"},
		{TokenSemicolon, ";"},
		{TokenTypeName, "HEADER"},
		{TokenSemicolon, ";"},
		{TokenTypeName, "ENDSEC"},
		{TokenSemicolon, ";"},
		{TokenTypeName, "DATA"},
		{TokenSemicolon, ";"},
		{TokenTypeName, "ENDSEC"},
		{TokenSemicolon, ";"},
		{TokenTypeName, "END-ISO-10303-21"},
		{TokenSemicolon, ";"},
		{TokenEOF, ""},
	}

	for i, exp := range expected {
		tok, err := lex.NextToken()
		if err != nil {
			t.Fatalf("token %d: unexpected error: %v", i, err)
		}
		if tok.Kind != exp.kind {
			t.Errorf("token %d: expected kind %v, got %v", i, exp.kind, tok.Kind)
		}
		if tok.Value != exp.value {
			t.Errorf("token %d: expected value %q, got %q", i, exp.value, tok.Value)
		}
	}
}
