package main

import "fmt"

type Lexer struct {
	input    string
	length   int
	position int
}

func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		length: len(input),
	}
}

func (l *Lexer) Tokenize() ([]Token, error) {
	var result []Token

	for l.position < l.length {
		current := l.Peek()

		if isWhitespace(current) {
			l.Next()
			continue
		}

		if isDigit(current) {
			l.tokenizeNumber(&result)
			continue
		}

		if isAlpha(current) {
			l.tokenizeWord(&result)
			continue
		}

		if current == '"' {
			if err := l.tokenizeString(&result); err != nil {
				return nil, err
			}
			continue
		}

		if err := l.tokenizeOperator(&result); err != nil {
			return nil, err
		}
	}

	result = append(result, Token{Type: TokenEOF, Value: "", Position: l.position})
	return result, nil
}

func (l *Lexer) tokenizeNumber(result *[]Token) {
	start := l.position

	for isDigit(l.Peek()) {
		l.Next()
	}

	numberStr := l.input[start:l.position]
	l.addToken(result, TokenNumber, numberStr, start)
}

func (l *Lexer) tokenizeWord(result *[]Token) {
	start := l.position

	for isAlphaNumeric(l.Peek()) {
		l.Next()
	}

	word := l.input[start:l.position]

	switch word {
	case "var":
		l.addToken(result, TokenVar, word, start)
	case "print":
		l.addToken(result, TokenPrint, word, start)
	case "if":
		l.addToken(result, TokenIf, word, start)
	case "else":
		l.addToken(result, TokenElse, word, start)
	case "while":
		l.addToken(result, TokenWhile, word, start)
	case "and":
		l.addToken(result, TokenAnd, word, start)
	case "or":
		l.addToken(result, TokenOr, word, start)
	case "string":
		l.addToken(result, TokenString, word, start)
	default:
		l.addToken(result, TokenID, word, start)
	}
}

func (l *Lexer) tokenizeString(result *[]Token) error {
	start := l.position
	l.Next()

	for l.position < l.length && l.Peek() != '"' {
		l.Next()
	}

	if l.position >= l.length {
		return fmt.Errorf("unterminated string at position %d", start)
	}

	l.Next()
	value := l.input[start+1 : l.position-1]
	l.addToken(result, TokenString, value, start)
	return nil
}

func (l *Lexer) tokenizeOperator(result *[]Token) error {
	current := l.Peek()
	start := l.position

	switch current {
	case '+':
		l.Next()
		l.addToken(result, TokenPlus, "+", start)
	case '-':
		l.Next()
		l.addToken(result, TokenMinus, "-", start)
	case '*':
		l.Next()
		l.addToken(result, TokenStar, "*", start)
	case '/':
		l.Next()
		l.addToken(result, TokenSlash, "/", start)
	case '=':
		l.Next()
		if l.Peek() == '=' {
			l.Next()
			l.addToken(result, TokenEqEq, "==", start)
		} else {
			l.addToken(result, TokenEq, "=", start)
		}
	case '!':
		l.Next()
		if l.Peek() == '=' {
			l.Next()
			l.addToken(result, TokenNeq, "!=", start)
		} else {
			l.addToken(result, TokenExcl, "!", start)
		}
	case '<':
		l.Next()
		if l.Peek() == '=' {
			l.Next()
			l.addToken(result, TokenLtEq, "<=", start)
		} else {
			l.addToken(result, TokenLt, "<", start)
		}
	case '>':
		l.Next()
		if l.Peek() == '=' {
			l.Next()
			l.addToken(result, TokenGtEq, ">=", start)
		} else {
			l.addToken(result, TokenGt, ">", start)
		}
	case '&':
		l.Next()
		if l.Peek() == '&' {
			l.Next()
			l.addToken(result, TokenAnd, "&&", start)
		} else {
			return fmt.Errorf("unexpected character '&' at position %d", start)
		}
	case '|':
		l.Next()
		if l.Peek() == '|' {
			l.Next()
			l.addToken(result, TokenOr, "||", start)
		} else {
			return fmt.Errorf("unexpected character '|' at position %d", start)
		}
	case '(':
		l.Next()
		l.addToken(result, TokenLParen, "(", start)
	case ')':
		l.Next()
		l.addToken(result, TokenRParen, ")", start)
	case '{':
		l.Next()
		l.addToken(result, TokenLBrace, "{", start)
	case '}':
		l.Next()
		l.addToken(result, TokenRBrace, "}", start)
	case ';':
		l.Next()
		l.addToken(result, TokenSemicolon, ";", start)
	default:
		return fmt.Errorf("unexpected character '%c' at position %d", current, start)
	}

	return nil
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
	return ch
}

func (l *Lexer) addToken(result *[]Token, tokenType TokenType, value string, start int) {
	*result = append(*result, Token{
		Type:     tokenType,
		Value:    value,
		Position: start,
	})
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
