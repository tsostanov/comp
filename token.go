package main

import "fmt"

type TokenType int

const (
	TokenNumber TokenType = iota
	TokenID
	TokenString
	TokenVar
	TokenPrint
	TokenIf
	TokenElse
	TokenWhile
	TokenPlus
	TokenMinus
	TokenStar
	TokenSlash
	TokenEq
	TokenEqEq
	TokenExcl
	TokenNeq
	TokenLt
	TokenGt
	TokenLtEq
	TokenGtEq
	TokenAnd
	TokenOr
	TokenLParen
	TokenRParen
	TokenLBrace
	TokenRBrace
	TokenSemicolon
	TokenEOF
)

var tokenTypeNames = []string{
	"NUMBER",
	"ID",
	"STRING",
	"VAR",
	"PRINT",
	"IF",
	"ELSE",
	"WHILE",
	"PLUS",
	"MINUS",
	"STAR",
	"SLASH",
	"EQ",
	"EQEQ",
	"EXCL",
	"NEQ",
	"LT",
	"GT",
	"LTEQ",
	"GTEQ",
	"AND",
	"OR",
	"LPAREN",
	"RPAREN",
	"LBRACE",
	"RBRACE",
	"SEMICOLON",
	"EOF",
}

func (t TokenType) String() string {
	if int(t) < len(tokenTypeNames) {
		return tokenTypeNames[t]
	}
	return "UNKNOWN"
}

type Token struct {
	Type     TokenType
	Value    string
	Position int
	Line     int
	Column   int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(Type: %s, Value: %q) at %d:%d", t.Type.String(), t.Value, t.Line, t.Column)
}
