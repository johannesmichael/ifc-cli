package step

import "fmt"

// TokenKind identifies the type of a lexer token.
type TokenKind int

const (
	TokenEntityID  TokenKind = iota // #123 at statement start
	TokenTypeName                   // IFCWALL, IFCPROPERTYSINGLEVALUE
	TokenString                     // 'Hello world'
	TokenInteger                    // 42, -7
	TokenFloat                     // 3.14, 1.5E-3
	TokenEnum                      // .ELEMENT., .T., .F.
	TokenRef                       // #456 inside attribute list
	TokenNull                      // $
	TokenDerived                   // *
	TokenLParen                    // (
	TokenRParen                    // )
	TokenComma                     // ,
	TokenSemicolon                 // ;
	TokenEquals                    // =
	TokenEOF
)

var tokenKindNames = [...]string{
	TokenEntityID:  "EntityID",
	TokenTypeName:  "TypeName",
	TokenString:    "String",
	TokenInteger:   "Integer",
	TokenFloat:     "Float",
	TokenEnum:      "Enum",
	TokenRef:       "Ref",
	TokenNull:      "Null",
	TokenDerived:   "Derived",
	TokenLParen:    "LParen",
	TokenRParen:    "RParen",
	TokenComma:     "Comma",
	TokenSemicolon: "Semicolon",
	TokenEquals:    "Equals",
	TokenEOF:       "EOF",
}

func (k TokenKind) String() string {
	if int(k) < len(tokenKindNames) {
		return tokenKindNames[k]
	}
	return fmt.Sprintf("TokenKind(%d)", int(k))
}

// Token represents a single lexical token from a STEP file.
type Token struct {
	Kind  TokenKind
	Value string // raw text
	Pos   int    // byte offset in source
}
