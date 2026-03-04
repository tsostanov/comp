package main

type Expr interface {
	exprNode()
}

type Stmt interface {
	stmtNode()
}

type LiteralExpr struct {
	Token Token
}

func (LiteralExpr) exprNode() {}

type VariableExpr struct {
	Name Token
}

func (VariableExpr) exprNode() {}

type UnaryExpr struct {
	Operator Token
	Right    Expr
}

func (UnaryExpr) exprNode() {}

type BinaryExpr struct {
	Left     Expr
	Operator Token
	Right    Expr
}

func (BinaryExpr) exprNode() {}

type AssignExpr struct {
	Name  Token
	Value Expr
}

func (AssignExpr) exprNode() {}

type GroupingExpr struct {
	Expression Expr
}

func (GroupingExpr) exprNode() {}

type VarStmt struct {
	Name        Token
	Initializer Expr
}

func (VarStmt) stmtNode() {}

type PrintStmt struct {
	Expression Expr
}

func (PrintStmt) stmtNode() {}

type ExprStmt struct {
	Expression Expr
}

func (ExprStmt) stmtNode() {}

type BlockStmt struct {
	Statements []Stmt
}

func (BlockStmt) stmtNode() {}

type IfStmt struct {
	Condition  Expr
	ThenBranch Stmt
	ElseBranch Stmt
}

func (IfStmt) stmtNode() {}

type WhileStmt struct {
	Condition Expr
	Body      Stmt
}

func (WhileStmt) stmtNode() {}
