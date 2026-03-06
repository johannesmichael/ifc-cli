package step

import "fmt"

// Lexer tokenizes a STEP/IFC byte stream.
type Lexer struct {
	src []byte
	pos int
}

// NewLexer creates a new Lexer for the given source bytes.
func NewLexer(src []byte) *Lexer {
	return &Lexer{src: src}
}

func (l *Lexer) peek() byte {
	if l.isAtEnd() {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) advance() byte {
	b := l.src[l.pos]
	l.pos++
	return b
}

func (l *Lexer) isAtEnd() bool {
	return l.pos >= len(l.src)
}

// skipWhitespace advances past spaces, tabs, newlines, and carriage returns.
func (l *Lexer) skipWhitespace() {
	for !l.isAtEnd() {
		switch l.peek() {
		case ' ', '\t', '\n', '\r':
			l.pos++
		default:
			return
		}
	}
}

// skipComment skips a /* ... */ block comment. Called when "/*" has been detected.
func (l *Lexer) skipComment() error {
	start := l.pos - 2 // position of '/'
	for !l.isAtEnd() {
		if l.advance() == '*' && !l.isAtEnd() && l.peek() == '/' {
			l.pos++ // consume '/'
			return nil
		}
	}
	return fmt.Errorf("unterminated comment starting at byte %d", start)
}

// NextToken returns the next token from the source, or an error.
func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespace()

	if l.isAtEnd() {
		return Token{Kind: TokenEOF, Pos: l.pos}, nil
	}

	pos := l.pos
	ch := l.advance()

	switch ch {
	case '(':
		return Token{Kind: TokenLParen, Value: "(", Pos: pos}, nil
	case ')':
		return Token{Kind: TokenRParen, Value: ")", Pos: pos}, nil
	case ',':
		return Token{Kind: TokenComma, Value: ",", Pos: pos}, nil
	case ';':
		return Token{Kind: TokenSemicolon, Value: ";", Pos: pos}, nil
	case '=':
		return Token{Kind: TokenEquals, Value: "=", Pos: pos}, nil
	case '$':
		return Token{Kind: TokenNull, Value: "$", Pos: pos}, nil
	case '*':
		return Token{Kind: TokenDerived, Value: "*", Pos: pos}, nil
	case '/':
		if !l.isAtEnd() && l.peek() == '*' {
			l.pos++ // consume '*'
			if err := l.skipComment(); err != nil {
				return Token{}, err
			}
			return l.NextToken()
		}
		return Token{}, fmt.Errorf("unexpected character '/' at byte %d", pos)

	// TODO: Task 1.3 — Numbers (#-prefixed entity IDs, integers, floats)
	// TODO: Task 1.4 — Strings ('...')
	// TODO: Task 1.5 — Entity IDs and refs (#123)
	// TODO: Task 1.6 — Type names (IFCWALL) and enums (.ELEMENT.)

	default:
		return Token{}, fmt.Errorf("unexpected character %q at byte %d", ch, pos)
	}
}
