package step

import (
	"fmt"
	"io"
	"strconv"
)

// Parser consumes tokens from a Lexer and produces Entity structs.
type Parser struct {
	lexer *Lexer
	cur   Token
}

// NewParser creates a new Parser for the given STEP source bytes.
func NewParser(src []byte) *Parser {
	p := &Parser{lexer: NewLexer(src)}
	p.advance() // prime the first token
	return p
}

func (p *Parser) advance() error {
	tok, err := p.lexer.NextToken()
	if err != nil {
		return err
	}
	p.cur = tok
	return nil
}

func (p *Parser) expect(kind TokenKind) (Token, error) {
	if p.cur.Kind != kind {
		return Token{}, fmt.Errorf("expected %v, got %v (%q) at byte %d", kind, p.cur.Kind, p.cur.Value, p.cur.Pos)
	}
	tok := p.cur
	if err := p.advance(); err != nil {
		return Token{}, err
	}
	return tok, nil
}

// skipHeader advances past the header section (everything up to and including the DATA; keyword).
func (p *Parser) skipHeader() error {
	for {
		if p.cur.Kind == TokenEOF {
			return io.EOF
		}
		if p.cur.Kind == TokenTypeName && p.cur.Value == "DATA" {
			if err := p.advance(); err != nil {
				return err
			}
			// consume the semicolon after DATA
			if p.cur.Kind == TokenSemicolon {
				if err := p.advance(); err != nil {
					return err
				}
			}
			return nil
		}
		if err := p.advance(); err != nil {
			return err
		}
	}
}

// Next returns the next parsed Entity, or (nil, io.EOF) when done.
func (p *Parser) Next() (*Entity, error) {
	// On first call, skip the header section
	if p.cur.Kind == TokenTypeName && (p.cur.Value == "ISO-10303-21" || p.cur.Value == "HEADER") {
		if err := p.skipHeader(); err != nil {
			return nil, err
		}
	}

	// Check for end of data or file
	if p.cur.Kind == TokenEOF {
		return nil, io.EOF
	}
	if p.cur.Kind == TokenTypeName && p.cur.Value == "END-ISO-10303-21" {
		return nil, io.EOF
	}
	if p.cur.Kind == TokenTypeName && p.cur.Value == "ENDSEC" {
		return nil, io.EOF
	}

	// Parse: #ID = TYPENAME ( attrs ) ;
	refTok, err := p.expect(TokenRef)
	if err != nil {
		return nil, fmt.Errorf("parsing entity ID: %w", err)
	}
	id, err := strconv.ParseUint(refTok.Value[1:], 10, 64) // skip '#'
	if err != nil {
		return nil, fmt.Errorf("invalid entity ID %q: %w", refTok.Value, err)
	}

	if _, err := p.expect(TokenEquals); err != nil {
		return nil, fmt.Errorf("parsing entity #%d: %w", id, err)
	}

	typeTok, err := p.expect(TokenTypeName)
	if err != nil {
		return nil, fmt.Errorf("parsing entity #%d type: %w", id, err)
	}

	if _, err := p.expect(TokenLParen); err != nil {
		return nil, fmt.Errorf("parsing entity #%d: %w", id, err)
	}

	attrs, err := p.parseAttrList()
	if err != nil {
		return nil, fmt.Errorf("parsing entity #%d attrs: %w", id, err)
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, fmt.Errorf("parsing entity #%d: %w", id, err)
	}

	if _, err := p.expect(TokenSemicolon); err != nil {
		return nil, fmt.Errorf("parsing entity #%d: %w", id, err)
	}

	return &Entity{
		ID:    id,
		Type:  typeTok.Value,
		Attrs: attrs,
	}, nil
}

// parseAttrList parses a comma-separated list of attribute values.
// It does NOT consume the closing RParen.
func (p *Parser) parseAttrList() ([]StepValue, error) {
	if p.cur.Kind == TokenRParen {
		return nil, nil
	}

	var attrs []StepValue
	for {
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, val)

		if p.cur.Kind != TokenComma {
			break
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return attrs, nil
}

// parseValue parses a single attribute value.
func (p *Parser) parseValue() (StepValue, error) {
	switch p.cur.Kind {
	case TokenString:
		v := StepValue{Kind: KindString, Str: p.cur.Value}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenInteger:
		n, err := strconv.ParseInt(p.cur.Value, 10, 64)
		if err != nil {
			return StepValue{}, fmt.Errorf("invalid integer %q at byte %d: %w", p.cur.Value, p.cur.Pos, err)
		}
		v := StepValue{Kind: KindInteger, Int: n}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenFloat:
		f, err := strconv.ParseFloat(p.cur.Value, 64)
		if err != nil {
			return StepValue{}, fmt.Errorf("invalid float %q at byte %d: %w", p.cur.Value, p.cur.Pos, err)
		}
		v := StepValue{Kind: KindFloat, Float: f}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenEnum:
		v := StepValue{Kind: KindEnum, Str: p.cur.Value}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenRef:
		ref, err := strconv.ParseUint(p.cur.Value[1:], 10, 64) // skip '#'
		if err != nil {
			return StepValue{}, fmt.Errorf("invalid ref %q at byte %d: %w", p.cur.Value, p.cur.Pos, err)
		}
		v := StepValue{Kind: KindRef, Ref: ref}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenNull:
		v := StepValue{Kind: KindNull}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenDerived:
		v := StepValue{Kind: KindDerived}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		return v, nil

	case TokenLParen:
		// List value
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		items, err := p.parseAttrList()
		if err != nil {
			return StepValue{}, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return StepValue{}, fmt.Errorf("parsing list: %w", err)
		}
		if items == nil {
			items = []StepValue{}
		}
		return StepValue{Kind: KindList, List: items}, nil

	case TokenTypeName:
		// Typed value: TYPENAME(inner)
		typeName := p.cur.Value
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		if p.cur.Kind != TokenLParen {
			return StepValue{}, fmt.Errorf("expected '(' after type name %q at byte %d", typeName, p.cur.Pos)
		}
		if err := p.advance(); err != nil {
			return StepValue{}, err
		}
		inner, err := p.parseValue()
		if err != nil {
			return StepValue{}, fmt.Errorf("parsing typed value %s: %w", typeName, err)
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return StepValue{}, fmt.Errorf("parsing typed value %s: %w", typeName, err)
		}
		return StepValue{Kind: KindTyped, Str: typeName, Inner: &inner}, nil

	default:
		return StepValue{}, fmt.Errorf("unexpected token %v (%q) at byte %d", p.cur.Kind, p.cur.Value, p.cur.Pos)
	}
}
