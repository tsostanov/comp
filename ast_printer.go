package main

import (
	"fmt"
	"strings"
)

type AstPrinter struct {
}

func NewAstPrinter() *AstPrinter {
	return &AstPrinter{}
}

func (p *AstPrinter) Print(stmts []Stmt) string {
	var b strings.Builder
	b.WriteString("Root (Program)\n")
	for i, stmt := range stmts {
		p.printNode(&b, stmt, "", i == len(stmts)-1)
	}
	return b.String()
}

func (p *AstPrinter) printNode(b *strings.Builder, node any, indent string, isLast bool) {
	if node == nil {
		return
	}

	marker := "├── "
	if isLast {
		marker = "└── "
	}
	b.WriteString(indent)
	b.WriteString(marker)

	childIndent := indent + "│   "
	if isLast {
		childIndent = indent + "    "
	}

	switch n := node.(type) {
	case VarStmt:
		fmt.Fprintf(b, "VarStatement: %s\n", n.Name.Value)
		if n.Initializer != nil {
			p.printNode(b, n.Initializer, childIndent, true)
		}
	case PrintStmt:
		b.WriteString("PrintStatement\n")
		p.printNode(b, n.Expression, childIndent, true)
	case ExprStmt:
		b.WriteString("ExpressionStatement\n")
		p.printNode(b, n.Expression, childIndent, true)
	case BlockStmt:
		b.WriteString("BlockStatement\n")
		for i, stmt := range n.Statements {
			p.printNode(b, stmt, childIndent, i == len(n.Statements)-1)
		}
	case IfStmt:
		b.WriteString("IfStatement\n")
		p.printNode(b, n.Condition, childIndent, false)
		isThenLast := n.ElseBranch == nil
		p.printNode(b, n.ThenBranch, childIndent, isThenLast)
		if n.ElseBranch != nil {
			p.printNode(b, n.ElseBranch, childIndent, true)
		}
	case WhileStmt:
		b.WriteString("WhileStatement\n")
		p.printNode(b, n.Condition, childIndent, false)
		p.printNode(b, n.Body, childIndent, true)
	case BinaryExpr:
		fmt.Fprintf(b, "BinaryExpression: %s\n", n.Operator.Value)
		p.printNode(b, n.Left, childIndent, false)
		p.printNode(b, n.Right, childIndent, true)
	case AssignExpr:
		fmt.Fprintf(b, "AssignExpression: %s =\n", n.Name.Value)
		p.printNode(b, n.Value, childIndent, true)
	case UnaryExpr:
		fmt.Fprintf(b, "UnaryExpression: %s\n", n.Operator.Value)
		p.printNode(b, n.Right, childIndent, true)
	case LiteralExpr:
		fmt.Fprintf(b, "Literal: %s\n", formatLiteral(n.Token))
	case VariableExpr:
		fmt.Fprintf(b, "Variable: %s\n", n.Name.Value)
	case GroupingExpr:
		b.WriteString("Grouping\n")
		p.printNode(b, n.Expression, childIndent, true)
	default:
		fmt.Fprintf(b, "Unknown Node: %T\n", node)
	}
}

func formatLiteral(tok Token) string {
	if tok.Type == TokenString {
		return fmt.Sprintf("%q", tok.Value)
	}
	return tok.Value
}
