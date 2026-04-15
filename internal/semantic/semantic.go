package semantic

import (
	"comp/internal/ast"
	tok "comp/internal/token"
	"fmt"
)

type DiagnosticSeverity string

const (
	SeverityError   DiagnosticSeverity = "error"
	SeverityWarning DiagnosticSeverity = "warning"
)

type SemanticDiagnostic struct {
	Severity DiagnosticSeverity
	Message  string
	Line     int
	Column   int
}

func (d SemanticDiagnostic) String() string {
	return fmt.Sprintf("%s at %d:%d: %s", d.Severity, d.Line, d.Column, d.Message)
}

type SymbolFlags struct {
	Defined     bool
	Initialized bool
	Used        bool
}

type VariableInfo struct {
	Name       string
	DeclaredAt tok.Token
	Type       ast.ValueType
	Flags      SymbolFlags
}

type SemanticEnvironment struct {
	parent    *SemanticEnvironment
	variables map[string]*VariableInfo
}

func NewSemanticEnvironment(parent *SemanticEnvironment) *SemanticEnvironment {
	return &SemanticEnvironment{
		parent:    parent,
		variables: make(map[string]*VariableInfo),
	}
}

func (e *SemanticEnvironment) Parent() *SemanticEnvironment {
	return e.parent
}

func (e *SemanticEnvironment) DefineVariable(name tok.Token, variableType ast.ValueType) (*VariableInfo, bool) {
	if _, exists := e.variables[name.Value]; exists {
		return nil, false
	}

	variable := &VariableInfo{
		Name:       name.Value,
		DeclaredAt: name,
		Type:       variableType,
		Flags: SymbolFlags{
			Defined: true,
		},
	}
	e.variables[name.Value] = variable
	return variable, true
}

func (e *SemanticEnvironment) ResolveVariable(name string) *VariableInfo {
	for current := e; current != nil; current = current.parent {
		if variable, ok := current.variables[name]; ok {
			return variable
		}
	}
	return nil
}

func (e *SemanticEnvironment) IsVariableDefined(name string) bool {
	variable := e.ResolveVariable(name)
	return variable != nil && variable.Flags.Defined
}

type variableState struct {
	defined     bool
	initialized bool
	used        bool
}

type SemanticAnalyzer struct {
	environment *SemanticEnvironment
	variables   []*VariableInfo
	diagnostics []SemanticDiagnostic
}

func NewSemanticAnalyzer() *SemanticAnalyzer {
	return &SemanticAnalyzer{
		environment: NewSemanticEnvironment(nil),
	}
}

func (a *SemanticAnalyzer) Analyze(statements []ast.Stmt) []SemanticDiagnostic {
	for _, statement := range statements {
		a.VisitStatement(statement)
	}
	a.reportUnusedVariables()
	return a.diagnostics
}

func (a *SemanticAnalyzer) VisitStatement(statement ast.Stmt) {
	switch s := statement.(type) {
	case ast.VarStmt:
		declaredType := ast.TypeUnknown
		if s.DeclaredType != nil {
			declaredType = s.DeclaredType.Kind
		}

		initializerType := ast.TypeUnknown
		if s.Initializer != nil {
			initializerType = a.VisitExpression(s.Initializer)
		}

		variableType := declaredType
		if variableType == ast.TypeUnknown {
			variableType = initializerType
		}

		variable, ok := a.environment.DefineVariable(s.Name, variableType)
		if !ok {
			a.errorAt(s.Name, "variable "+s.Name.Value+" is already declared in this scope")
			return
		}
		a.variables = append(a.variables, variable)

		if s.DeclaredType == nil && s.Initializer == nil {
			a.errorAt(s.Name, "variable "+s.Name.Value+" requires an explicit type or initializer")
		}
		if declaredType != ast.TypeUnknown && initializerType != ast.TypeUnknown && !a.isAssignable(declaredType, initializerType) {
			a.errorAt(s.Name, "cannot initialize variable "+s.Name.Value+" of type "+declaredType.String()+" with value of type "+initializerType.String())
		}
		if s.Initializer != nil {
			variable.Flags.Initialized = true
		}
	case ast.PrintStmt:
		a.VisitExpression(s.Expression)
	case ast.ExprStmt:
		a.VisitExpression(s.Expression)
	case ast.BlockStmt:
		previousEnvironment := a.environment
		a.environment = NewSemanticEnvironment(previousEnvironment)

		for _, nested := range s.Statements {
			a.VisitStatement(nested)
		}

		a.environment = previousEnvironment
	case ast.IfStmt:
		conditionType := a.VisitExpression(s.Condition)
		a.requireType(conditionType, ast.TypeBool, s.Condition, "if condition must have type bool")
		before := a.snapshotVariableStates()

		a.VisitStatement(s.ThenBranch)
		thenState := a.snapshotVariableStates()

		if s.ElseBranch == nil {
			a.restoreVariableStates(before)
			a.mergeVariableStates(before, thenState, nil)
			return
		}

		a.restoreVariableStates(before)
		a.VisitStatement(s.ElseBranch)
		elseState := a.snapshotVariableStates()
		a.restoreVariableStates(before)
		a.mergeVariableStates(before, thenState, elseState)
	case ast.WhileStmt:
		conditionType := a.VisitExpression(s.Condition)
		a.requireType(conditionType, ast.TypeBool, s.Condition, "while condition must have type bool")
		before := a.snapshotVariableStates()

		a.VisitStatement(s.Body)
		bodyState := a.snapshotVariableStates()
		a.restoreVariableStates(before)

		// Loop body may never execute, so initialization is not guaranteed afterwards.
		for variable, state := range before {
			body := bodyState[variable]
			variable.Flags.Defined = state.defined
			variable.Flags.Initialized = state.initialized
			variable.Flags.Used = state.used || body.used
		}
	}
}

func (a *SemanticAnalyzer) VisitExpression(expression ast.Expr) ast.ValueType {
	switch e := expression.(type) {
	case ast.LiteralExpr:
		return literalType(e.Token)
	case ast.VariableExpr:
		variable := a.environment.ResolveVariable(e.Name.Value)
		if variable == nil || !variable.Flags.Defined {
			a.errorAt(e.Name, "use of undeclared variable "+e.Name.Value)
			return ast.TypeUnknown
		}

		variable.Flags.Used = true
		if !variable.Flags.Initialized {
			a.errorAt(e.Name, "variable "+e.Name.Value+" is used before initialization")
		}
		return variable.Type
	case ast.AssignExpr:
		valueType := a.VisitExpression(e.Value)

		variable := a.environment.ResolveVariable(e.Name.Value)
		if variable == nil || !variable.Flags.Defined {
			a.errorAt(e.Name, "assignment to undeclared variable "+e.Name.Value)
			return ast.TypeUnknown
		}
		if variable.Type != ast.TypeUnknown && valueType != ast.TypeUnknown && !a.isAssignable(variable.Type, valueType) {
			a.errorAt(e.Name, "cannot assign value of type "+valueType.String()+" to variable "+e.Name.Value+" of type "+variable.Type.String())
		}

		variable.Flags.Initialized = true
		return variable.Type
	case ast.BinaryExpr:
		leftType := a.VisitExpression(e.Left)
		rightType := a.VisitExpression(e.Right)
		return a.checkBinaryExpression(e, leftType, rightType)
	case ast.UnaryExpr:
		rightType := a.VisitExpression(e.Right)
		return a.checkUnaryExpression(e, rightType)
	case ast.GroupingExpr:
		return a.VisitExpression(e.Expression)
	}
	return ast.TypeUnknown
}

func (a *SemanticAnalyzer) HasErrors() bool {
	for _, diagnostic := range a.diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (a *SemanticAnalyzer) Diagnostics() []SemanticDiagnostic {
	return a.diagnostics
}

func (a *SemanticAnalyzer) snapshotVariableStates() map[*VariableInfo]variableState {
	snapshot := make(map[*VariableInfo]variableState, len(a.variables))
	for _, variable := range a.variables {
		snapshot[variable] = variableState{
			defined:     variable.Flags.Defined,
			initialized: variable.Flags.Initialized,
			used:        variable.Flags.Used,
		}
	}
	return snapshot
}

func (a *SemanticAnalyzer) restoreVariableStates(snapshot map[*VariableInfo]variableState) {
	for variable, state := range snapshot {
		variable.Flags.Defined = state.defined
		variable.Flags.Initialized = state.initialized
		variable.Flags.Used = state.used
	}
}

func (a *SemanticAnalyzer) mergeVariableStates(before, left, right map[*VariableInfo]variableState) {
	for variable, state := range before {
		leftState := left[variable]
		if right == nil {
			variable.Flags.Defined = state.defined
			variable.Flags.Initialized = state.initialized
			variable.Flags.Used = state.used || leftState.used
			continue
		}

		rightState := right[variable]
		variable.Flags.Defined = leftState.defined && rightState.defined
		variable.Flags.Initialized = leftState.initialized && rightState.initialized
		variable.Flags.Used = leftState.used || rightState.used
	}
}

func (a *SemanticAnalyzer) reportUnusedVariables() {
	for _, variable := range a.variables {
		if !variable.Flags.Defined || variable.Flags.Used {
			continue
		}

		a.diagnostics = append(a.diagnostics, SemanticDiagnostic{
			Severity: SeverityWarning,
			Message:  "variable " + variable.Name + " is declared but never used",
			Line:     variable.DeclaredAt.Line,
			Column:   variable.DeclaredAt.Column,
		})
	}
}

func (a *SemanticAnalyzer) errorAt(token tok.Token, message string) {
	a.diagnostics = append(a.diagnostics, SemanticDiagnostic{
		Severity: SeverityError,
		Message:  message,
		Line:     token.Line,
		Column:   token.Column,
	})
}

func (a *SemanticAnalyzer) requireType(actual, expected ast.ValueType, expression ast.Expr, message string) {
	if actual == ast.TypeUnknown || actual == expected {
		return
	}
	a.errorAt(expressionToken(expression), message)
}

func (a *SemanticAnalyzer) isAssignable(target, value ast.ValueType) bool {
	if target == ast.TypeUnknown || value == ast.TypeUnknown {
		return true
	}
	return target == value
}

func (a *SemanticAnalyzer) checkUnaryExpression(expression ast.UnaryExpr, rightType ast.ValueType) ast.ValueType {
	switch expression.Operator.Type {
	case tok.TokenMinus:
		if rightType != ast.TypeUnknown && rightType != ast.TypeInt {
			a.errorAt(expression.Operator, "operator - expects operand of type int")
		}
		return ast.TypeInt
	case tok.TokenExcl:
		if rightType != ast.TypeUnknown && rightType != ast.TypeBool {
			a.errorAt(expression.Operator, "operator ! expects operand of type bool")
		}
		return ast.TypeBool
	default:
		return ast.TypeUnknown
	}
}

func (a *SemanticAnalyzer) checkBinaryExpression(expression ast.BinaryExpr, leftType, rightType ast.ValueType) ast.ValueType {
	switch expression.Operator.Type {
	case tok.TokenPlus:
		if leftType == ast.TypeUnknown || rightType == ast.TypeUnknown {
			return ast.TypeUnknown
		}
		if leftType == ast.TypeInt && rightType == ast.TypeInt {
			return ast.TypeInt
		}
		if leftType == ast.TypeString && rightType == ast.TypeString {
			return ast.TypeString
		}
		a.errorAt(expression.Operator, "operator + expects operands of type int or string")
		return ast.TypeUnknown
	case tok.TokenMinus, tok.TokenStar, tok.TokenSlash:
		if leftType != ast.TypeUnknown && leftType != ast.TypeInt {
			a.errorAt(expression.Operator, "arithmetic operators expect operands of type int")
		}
		if rightType != ast.TypeUnknown && rightType != ast.TypeInt {
			a.errorAt(expression.Operator, "arithmetic operators expect operands of type int")
		}
		return ast.TypeInt
	case tok.TokenAnd, tok.TokenOr:
		if leftType != ast.TypeUnknown && leftType != ast.TypeBool {
			a.errorAt(expression.Operator, "logical operators expect operands of type bool")
		}
		if rightType != ast.TypeUnknown && rightType != ast.TypeBool {
			a.errorAt(expression.Operator, "logical operators expect operands of type bool")
		}
		return ast.TypeBool
	case tok.TokenLt, tok.TokenLtEq, tok.TokenGt, tok.TokenGtEq:
		if leftType != ast.TypeUnknown && leftType != ast.TypeInt {
			a.errorAt(expression.Operator, "comparison operators expect operands of type int")
		}
		if rightType != ast.TypeUnknown && rightType != ast.TypeInt {
			a.errorAt(expression.Operator, "comparison operators expect operands of type int")
		}
		return ast.TypeBool
	case tok.TokenEqEq, tok.TokenNeq:
		if leftType != ast.TypeUnknown && rightType != ast.TypeUnknown && leftType != rightType {
			a.errorAt(expression.Operator, "equality operators require operands of the same type")
		}
		return ast.TypeBool
	default:
		return ast.TypeUnknown
	}
}

func literalType(token tok.Token) ast.ValueType {
	switch token.Type {
	case tok.TokenNumber:
		return ast.TypeInt
	case tok.TokenString:
		return ast.TypeString
	case tok.TokenTrue, tok.TokenFalse:
		return ast.TypeBool
	default:
		return ast.TypeUnknown
	}
}

func expressionToken(expression ast.Expr) tok.Token {
	switch e := expression.(type) {
	case ast.LiteralExpr:
		return e.Token
	case ast.VariableExpr:
		return e.Name
	case ast.UnaryExpr:
		return e.Operator
	case ast.BinaryExpr:
		return e.Operator
	case ast.AssignExpr:
		return e.Name
	case ast.GroupingExpr:
		return expressionToken(e.Expression)
	default:
		return tok.Token{}
	}
}
