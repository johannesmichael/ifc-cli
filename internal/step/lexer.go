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

	case '\'':
		return l.readString(pos)

	case '#':
		return l.readEntityRef(pos)

	case '.':
		if !l.isAtEnd() && isUpperOrDigitOrUnderscore(l.peek()) {
			return l.readEnum(pos)
		}
		return Token{}, fmt.Errorf("unexpected character '.' at byte %d", pos)

	default:
		if isDigit(ch) {
			return l.readNumber(pos, ch)
		}
		if ch == '+' || ch == '-' {
			if !l.isAtEnd() && (isDigit(l.peek()) || l.peek() == '.') {
				return l.readNumber(pos, ch)
			}
			return Token{}, fmt.Errorf("unexpected character %q at byte %d", ch, pos)
		}
		if isUpperLetter(ch) {
			return l.readTypeName(pos)
		}
		return Token{}, fmt.Errorf("unexpected character %q at byte %d", ch, pos)
	}
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isUpperLetter(ch byte) bool {
	return ch >= 'A' && ch <= 'Z'
}

func isUpperOrDigitOrUnderscore(ch byte) bool {
	return isUpperLetter(ch) || isDigit(ch) || ch == '_'
}

// readNumber parses an integer or float token. The first character (digit or sign) has already been consumed.
func (l *Lexer) readNumber(pos int, first byte) (Token, error) {
	start := pos
	hasDecimal := false
	hasExponent := false

	// If first char is sign and next is '.', handle leading decimal
	if (first == '+' || first == '-') && !l.isAtEnd() && l.peek() == '.' {
		l.pos++
		hasDecimal = true
	}

	for !l.isAtEnd() {
		ch := l.peek()
		if isDigit(ch) {
			l.pos++
		} else if ch == '.' && !hasDecimal && !hasExponent {
			hasDecimal = true
			l.pos++
		} else if (ch == 'E' || ch == 'e') && !hasExponent {
			hasExponent = true
			hasDecimal = true // exponent implies float
			l.pos++
			if !l.isAtEnd() && (l.peek() == '+' || l.peek() == '-') {
				l.pos++
			}
		} else {
			break
		}
	}

	raw := string(l.src[start:l.pos])
	if hasDecimal || hasExponent {
		return Token{Kind: TokenFloat, Value: raw, Pos: start}, nil
	}
	return Token{Kind: TokenInteger, Value: raw, Pos: start}, nil
}

// readString parses a single-quoted string. The opening quote has been consumed.
func (l *Lexer) readString(pos int) (Token, error) {
	var content []byte
	for {
		if l.isAtEnd() {
			return Token{}, fmt.Errorf("unterminated string starting at byte %d", pos)
		}
		ch := l.advance()
		if ch == '\'' {
			if !l.isAtEnd() && l.peek() == '\'' {
				content = append(content, '\'')
				l.pos++
			} else {
				return Token{Kind: TokenString, Value: string(content), Pos: pos}, nil
			}
		} else {
			content = append(content, ch)
		}
	}
}

// readEntityRef parses #NNN. The '#' has been consumed.
func (l *Lexer) readEntityRef(pos int) (Token, error) {
	if l.isAtEnd() || !isDigit(l.peek()) {
		return Token{}, fmt.Errorf("expected digits after '#' at byte %d", pos)
	}
	for !l.isAtEnd() && isDigit(l.peek()) {
		l.pos++
	}
	return Token{Kind: TokenRef, Value: string(l.src[pos:l.pos]), Pos: pos}, nil
}

// readEnum parses .NAME. — the leading '.' has been consumed.
func (l *Lexer) readEnum(pos int) (Token, error) {
	for !l.isAtEnd() && isUpperOrDigitOrUnderscore(l.peek()) {
		l.pos++
	}
	if l.isAtEnd() || l.peek() != '.' {
		return Token{}, fmt.Errorf("unterminated enum starting at byte %d", pos)
	}
	l.pos++ // consume trailing '.'
	return Token{Kind: TokenEnum, Value: string(l.src[pos:l.pos]), Pos: pos}, nil
}

// readTypeName parses an uppercase identifier. The first letter has been consumed.
func (l *Lexer) readTypeName(pos int) (Token, error) {
	for !l.isAtEnd() && (isUpperOrDigitOrUnderscore(l.peek()) || l.peek() == '-') {
		l.pos++
	}
	return Token{Kind: TokenTypeName, Value: string(l.src[pos:l.pos]), Pos: pos}, nil
}
