package main

import "fmt"

type Lexer struct {
	input    string
	length   int
	position int
	line     int
	column   int
}

func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		length: len(input),
		line:   1,
		column: 1,
	}
}

func (l *Lexer) Tokenize() ([]Token, error) {
	var result []Token
	err := l.TokenizeEach(func(tok Token) bool {
		result = append(result, tok)
		return true
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (l *Lexer) TokenizeEach(yield func(Token) bool) error {
	for l.position < l.length {
		current := l.Peek()

		if isWhitespace(current) {
			l.Next()
			continue
		}

		if isDigit(current) {
			if !yield(l.readNumber()) {
				return nil
			}
			continue
		}

		if isAlpha(current) {
			if !yield(l.readWord()) {
				return nil
			}
			continue
		}

		if current == '"' {
			tok, err := l.readString()
			if err != nil {
				return err
			}
			if !yield(tok) {
				return nil
			}
			continue
		}

		tok, err := l.readOperatorOrPunctuation()
		if err != nil {
			return err
		}
		if !yield(tok) {
			return nil
		}
	}

	eofToken := Token{
		Type:     TokenEOF,
		Value:    "",
		Position: l.position,
		Line:     l.line,
		Column:   l.column,
	}
	yield(eofToken)
	return nil
}

var keywords = map[string]TokenType{
	"var":    TokenVar,
	"print":  TokenPrint,
	"if":     TokenIf,
	"else":   TokenElse,
	"while":  TokenWhile,
	"and":    TokenAnd,
	"or":     TokenOr,
	"string": TokenString,
}

var operators = map[string]TokenType{
	"==": TokenEqEq,
	"!=": TokenNeq,
	"<=": TokenLtEq,
	">=": TokenGtEq,
	"&&": TokenAnd,
	"||": TokenOr,
	"+":  TokenPlus,
	"-":  TokenMinus,
	"*":  TokenStar,
	"/":  TokenSlash,
	"=":  TokenEq,
	"<":  TokenLt,
	">":  TokenGt,
	"!":  TokenExcl,
	"(":  TokenLParen,
	")":  TokenRParen,
	"{":  TokenLBrace,
	"}":  TokenRBrace,
	";":  TokenSemicolon,
}

func (l *Lexer) readNumber() Token {
	start := l.position
	startLine := l.line
	startCol := l.column

	for isDigit(l.Peek()) {
		l.Next()
	}

	numberStr := l.input[start:l.position]
	return Token{
		Type:     TokenNumber,
		Value:    numberStr,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}
}

func (l *Lexer) readWord() Token {
	start := l.position
	startLine := l.line
	startCol := l.column

	for isAlphaNumeric(l.Peek()) {
		l.Next()
	}

	word := l.input[start:l.position]
	tokenType := TokenID
	if kwType, ok := keywords[word]; ok {
		tokenType = kwType
	}

	return Token{
		Type:     tokenType,
		Value:    word,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}
}

func (l *Lexer) readString() (Token, error) {
	start := l.position
	startLine := l.line
	startCol := l.column
	l.Next()

	for l.position < l.length && l.Peek() != '"' {
		l.Next()
	}

	if l.position >= l.length {
		return Token{}, fmt.Errorf("unterminated string at %d:%d", startLine, startCol)
	}

	l.Next()
	value := l.input[start+1 : l.position-1]
	return Token{
		Type:     TokenString,
		Value:    value,
		Position: start,
		Line:     startLine,
		Column:   startCol,
	}, nil
}

func (l *Lexer) readOperatorOrPunctuation() (Token, error) {
	start := l.position
	startLine := l.line
	startCol := l.column

	if l.position+1 < l.length {
		twoChars := l.input[l.position : l.position+2]
		if tokenType, ok := operators[twoChars]; ok {
			l.Next()
			l.Next()
			return Token{
				Type:     tokenType,
				Value:    twoChars,
				Position: start,
				Line:     startLine,
				Column:   startCol,
			}, nil
		}
	}

	oneChar := l.input[l.position : l.position+1]
	if tokenType, ok := operators[oneChar]; ok {
		l.Next()
		return Token{
			Type:     tokenType,
			Value:    oneChar,
			Position: start,
			Line:     startLine,
			Column:   startCol,
		}, nil
	}

	return Token{}, fmt.Errorf("unexpected character '%c' at %d:%d", l.Peek(), startLine, startCol)
}

func (l *Lexer) Peek() byte {
	if l.position >= l.length {
		return 0
	}
	return l.input[l.position]
}

func (l *Lexer) Next() byte {
	if l.position >= l.length {
		return 0
	}
	ch := l.input[l.position]
	l.position++
	if ch == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return ch
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\r' || ch == '\t' || ch == '\n'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isAlphaNumeric(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}
