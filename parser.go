package main

import "fmt"

type ParseError struct {
	Message string
	Line    int
	Column  int
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d: %s", e.Line, e.Column, e.Message)
}

type Parser struct {
	tokens   []Token
	position int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{
		tokens: tokens,
	}
}

func (p *Parser) Parse() ([]Stmt, error) {
	var statements []Stmt
	for !p.isAtEnd() {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}
	return statements, nil
}

func (p *Parser) parseStatement() (Stmt, error) {
	if p.match(TokenVar) {
		return p.parseVarDeclaration()
	}
	if p.match(TokenPrint) {
		return p.parsePrintStatement()
	}
	if p.match(TokenIf) {
		return p.parseIfStatement()
	}
	if p.match(TokenWhile) {
		return p.parseWhileStatement()
	}
	if p.match(TokenLBrace) {
		return p.parseBlockStatement()
	}
	return p.parseExpressionStatement()
}

func (p *Parser) parseVarDeclaration() (Stmt, error) {
	name, err := p.consume(TokenID, "expected variable name")
	if err != nil {
		return nil, err
	}

	var declaredType *TypeAnnotation
	if p.match(TokenColon) {
		declaredType, err = p.parseTypeAnnotation()
		if err != nil {
			return nil, err
		}
	}

	var initializer Expr
	if p.match(TokenEq) {
		initializer, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	if _, err := p.consume(TokenSemicolon, "expected ';' after variable declaration"); err != nil {
		return nil, err
	}

	return VarStmt{Name: name, DeclaredType: declaredType, Initializer: initializer}, nil
}

func (p *Parser) parsePrintStatement() (Stmt, error) {
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.consume(TokenSemicolon, "expected ';' after print statement"); err != nil {
		return nil, err
	}
	return PrintStmt{Expression: expr}, nil
}

func (p *Parser) parseIfStatement() (Stmt, error) {
	if _, err := p.consume(TokenLParen, "expected '(' after 'if'"); err != nil {
		return nil, err
	}
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.consume(TokenRParen, "expected ')' after if condition"); err != nil {
		return nil, err
	}

	thenBranch, err := p.parseStatement()
	if err != nil {
		return nil, err
	}

	var elseBranch Stmt
	if p.match(TokenElse) {
		elseBranch, err = p.parseStatement()
		if err != nil {
			return nil, err
		}
	}

	return IfStmt{Condition: condition, ThenBranch: thenBranch, ElseBranch: elseBranch}, nil
}

func (p *Parser) parseWhileStatement() (Stmt, error) {
	if _, err := p.consume(TokenLParen, "expected '(' after 'while'"); err != nil {
		return nil, err
	}
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.consume(TokenRParen, "expected ')' after while condition"); err != nil {
		return nil, err
	}

	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}

	return WhileStmt{Condition: condition, Body: body}, nil
}

func (p *Parser) parseBlockStatement() (Stmt, error) {
	var statements []Stmt
	for !p.check(TokenRBrace) && !p.isAtEnd() {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}
	if _, err := p.consume(TokenRBrace, "expected '}' after block"); err != nil {
		return nil, err
	}
	return BlockStmt{Statements: statements}, nil
}

func (p *Parser) parseExpressionStatement() (Stmt, error) {
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.consume(TokenSemicolon, "expected ';' after expression"); err != nil {
		return nil, err
	}
	return ExprStmt{Expression: expr}, nil
}

func (p *Parser) parseExpression() (Expr, error) {
	return p.parseAssignment()
}

func (p *Parser) parseAssignment() (Expr, error) {
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if p.match(TokenEq) {
		equals := p.previous()
		value, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}

		if varExpr, ok := expr.(VariableExpr); ok {
			return AssignExpr{Name: varExpr.Name, Value: value}, nil
		}

		return nil, ParseError{
			Message: "invalid assignment target",
			Line:    equals.Line,
			Column:  equals.Column,
		}
	}

	return expr, nil
}

func (p *Parser) parseOr() (Expr, error) {
	expr, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.match(TokenOr) {
		operator := p.previous()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		expr = BinaryExpr{Left: expr, Operator: operator, Right: right}
	}
	return expr, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	expr, err := p.parseEquality()
	if err != nil {
		return nil, err
	}

	for p.match(TokenAnd) {
		operator := p.previous()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		expr = BinaryExpr{Left: expr, Operator: operator, Right: right}
	}
	return expr, nil
}

func (p *Parser) parseEquality() (Expr, error) {
	expr, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	for p.match(TokenEqEq, TokenNeq) {
		operator := p.previous()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		expr = BinaryExpr{Left: expr, Operator: operator, Right: right}
	}
	return expr, nil
}

func (p *Parser) parseComparison() (Expr, error) {
	expr, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

	for p.match(TokenLt, TokenLtEq, TokenGt, TokenGtEq) {
		operator := p.previous()
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		expr = BinaryExpr{Left: expr, Operator: operator, Right: right}
	}
	return expr, nil
}

func (p *Parser) parseTerm() (Expr, error) {
	expr, err := p.parseFactor()
	if err != nil {
		return nil, err
	}

	for p.match(TokenPlus, TokenMinus) {
		operator := p.previous()
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		expr = BinaryExpr{Left: expr, Operator: operator, Right: right}
	}
	return expr, nil
}

func (p *Parser) parseFactor() (Expr, error) {
	expr, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.match(TokenStar, TokenSlash) {
		operator := p.previous()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		expr = BinaryExpr{Left: expr, Operator: operator, Right: right}
	}
	return expr, nil
}

func (p *Parser) parseUnary() (Expr, error) {
	if p.match(TokenExcl, TokenMinus) {
		operator := p.previous()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return UnaryExpr{Operator: operator, Right: right}, nil
	}
	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Expr, error) {
	if p.match(TokenNumber, TokenString, TokenTrue, TokenFalse) {
		return LiteralExpr{Token: p.previous()}, nil
	}
	if p.match(TokenID) {
		return VariableExpr{Name: p.previous()}, nil
	}
	if p.match(TokenLParen) {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.consume(TokenRParen, "expected ')' after expression"); err != nil {
			return nil, err
		}
		return GroupingExpr{Expression: expr}, nil
	}

	token := p.peek()
	return nil, ParseError{
		Message: "expected expression",
		Line:    token.Line,
		Column:  token.Column,
	}
}

func (p *Parser) parseTypeAnnotation() (*TypeAnnotation, error) {
	switch {
	case p.match(TokenInt):
		token := p.previous()
		return &TypeAnnotation{Name: token, Kind: TypeInt}, nil
	case p.match(TokenBool):
		token := p.previous()
		return &TypeAnnotation{Name: token, Kind: TypeBool}, nil
	case p.match(TokenStringType):
		token := p.previous()
		return &TypeAnnotation{Name: token, Kind: TypeString}, nil
	default:
		token := p.peek()
		return nil, ParseError{
			Message: "expected type name",
			Line:    token.Line,
			Column:  token.Column,
		}
	}
}

func (p *Parser) match(types ...TokenType) bool {
	for _, t := range types {
		if p.check(t) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) consume(t TokenType, message string) (Token, error) {
	if p.check(t) {
		return p.advance(), nil
	}
	token := p.peek()
	return Token{}, ParseError{
		Message: message,
		Line:    token.Line,
		Column:  token.Column,
	}
}

func (p *Parser) check(t TokenType) bool {
	if p.isAtEnd() {
		return false
	}
	return p.peek().Type == t
}

func (p *Parser) checkNext(t TokenType) bool {
	if p.position+1 >= len(p.tokens) {
		return false
	}
	return p.tokens[p.position+1].Type == t
}

func (p *Parser) advance() Token {
	if !p.isAtEnd() {
		p.position++
	}
	return p.previous()
}

func (p *Parser) isAtEnd() bool {
	return p.peek().Type == TokenEOF
}

func (p *Parser) peek() Token {
	if p.position >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.position]
}

func (p *Parser) previous() Token {
	if p.position-1 < 0 || p.position-1 >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.position-1]
}
