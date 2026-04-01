package main

import "fmt"

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
	DeclaredAt Token
	Type       ValueType
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

func (e *SemanticEnvironment) DefineVariable(name Token, variableType ValueType) (*VariableInfo, bool) {
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

func (a *SemanticAnalyzer) Analyze(statements []Stmt) []SemanticDiagnostic {
	for _, statement := range statements {
		a.VisitStatement(statement)
	}
	a.reportUnusedVariables()
	return a.diagnostics
}

func (a *SemanticAnalyzer) VisitStatement(statement Stmt) {
	switch s := statement.(type) {
	case VarStmt:
		declaredType := TypeUnknown
		if s.DeclaredType != nil {
			declaredType = s.DeclaredType.Kind
		}

		initializerType := TypeUnknown
		if s.Initializer != nil {
			initializerType = a.VisitExpression(s.Initializer)
		}

		variableType := declaredType
		if variableType == TypeUnknown {
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
		if declaredType != TypeUnknown && initializerType != TypeUnknown && !a.isAssignable(declaredType, initializerType) {
			a.errorAt(s.Name, "cannot initialize variable "+s.Name.Value+" of type "+declaredType.String()+" with value of type "+initializerType.String())
		}
		if s.Initializer != nil {
			variable.Flags.Initialized = true
		}
	case PrintStmt:
		a.VisitExpression(s.Expression)
	case ExprStmt:
		a.VisitExpression(s.Expression)
	case BlockStmt:
		previousEnvironment := a.environment
		a.environment = NewSemanticEnvironment(previousEnvironment)

		for _, nested := range s.Statements {
			a.VisitStatement(nested)
		}

		a.environment = previousEnvironment
	case IfStmt:
		conditionType := a.VisitExpression(s.Condition)
		a.requireType(conditionType, TypeBool, s.Condition, "if condition must have type bool")
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
	case WhileStmt:
		conditionType := a.VisitExpression(s.Condition)
		a.requireType(conditionType, TypeBool, s.Condition, "while condition must have type bool")
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

func (a *SemanticAnalyzer) VisitExpression(expression Expr) ValueType {
	switch e := expression.(type) {
	case LiteralExpr:
		return literalType(e.Token)
	case VariableExpr:
		variable := a.environment.ResolveVariable(e.Name.Value)
		if variable == nil || !variable.Flags.Defined {
			a.errorAt(e.Name, "use of undeclared variable "+e.Name.Value)
			return TypeUnknown
		}

		variable.Flags.Used = true
		if !variable.Flags.Initialized {
			a.errorAt(e.Name, "variable "+e.Name.Value+" is used before initialization")
		}
		return variable.Type
	case AssignExpr:
		valueType := a.VisitExpression(e.Value)

		variable := a.environment.ResolveVariable(e.Name.Value)
		if variable == nil || !variable.Flags.Defined {
			a.errorAt(e.Name, "assignment to undeclared variable "+e.Name.Value)
			return TypeUnknown
		}
		if variable.Type != TypeUnknown && valueType != TypeUnknown && !a.isAssignable(variable.Type, valueType) {
			a.errorAt(e.Name, "cannot assign value of type "+valueType.String()+" to variable "+e.Name.Value+" of type "+variable.Type.String())
		}

		variable.Flags.Initialized = true
		return variable.Type
	case BinaryExpr:
		leftType := a.VisitExpression(e.Left)
		rightType := a.VisitExpression(e.Right)
		return a.checkBinaryExpression(e, leftType, rightType)
	case UnaryExpr:
		rightType := a.VisitExpression(e.Right)
		return a.checkUnaryExpression(e, rightType)
	case GroupingExpr:
		return a.VisitExpression(e.Expression)
	}
	return TypeUnknown
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

func (a *SemanticAnalyzer) errorAt(token Token, message string) {
	a.diagnostics = append(a.diagnostics, SemanticDiagnostic{
		Severity: SeverityError,
		Message:  message,
		Line:     token.Line,
		Column:   token.Column,
	})
}

func (a *SemanticAnalyzer) requireType(actual, expected ValueType, expression Expr, message string) {
	if actual == TypeUnknown || actual == expected {
		return
	}
	a.errorAt(expressionToken(expression), message)
}

func (a *SemanticAnalyzer) isAssignable(target, value ValueType) bool {
	if target == TypeUnknown || value == TypeUnknown {
		return true
	}
	return target == value
}

func (a *SemanticAnalyzer) checkUnaryExpression(expression UnaryExpr, rightType ValueType) ValueType {
	switch expression.Operator.Type {
	case TokenMinus:
		if rightType != TypeUnknown && rightType != TypeInt {
			a.errorAt(expression.Operator, "operator - expects operand of type int")
		}
		return TypeInt
	case TokenExcl:
		if rightType != TypeUnknown && rightType != TypeBool {
			a.errorAt(expression.Operator, "operator ! expects operand of type bool")
		}
		return TypeBool
	default:
		return TypeUnknown
	}
}

func (a *SemanticAnalyzer) checkBinaryExpression(expression BinaryExpr, leftType, rightType ValueType) ValueType {
	switch expression.Operator.Type {
	case TokenPlus:
		if leftType == TypeUnknown || rightType == TypeUnknown {
			return TypeUnknown
		}
		if leftType == TypeInt && rightType == TypeInt {
			return TypeInt
		}
		if leftType == TypeString && rightType == TypeString {
			return TypeString
		}
		a.errorAt(expression.Operator, "operator + expects operands of type int or string")
		return TypeUnknown
	case TokenMinus, TokenStar, TokenSlash:
		if leftType != TypeUnknown && leftType != TypeInt {
			a.errorAt(expression.Operator, "arithmetic operators expect operands of type int")
		}
		if rightType != TypeUnknown && rightType != TypeInt {
			a.errorAt(expression.Operator, "arithmetic operators expect operands of type int")
		}
		return TypeInt
	case TokenAnd, TokenOr:
		if leftType != TypeUnknown && leftType != TypeBool {
			a.errorAt(expression.Operator, "logical operators expect operands of type bool")
		}
		if rightType != TypeUnknown && rightType != TypeBool {
			a.errorAt(expression.Operator, "logical operators expect operands of type bool")
		}
		return TypeBool
	case TokenLt, TokenLtEq, TokenGt, TokenGtEq:
		if leftType != TypeUnknown && leftType != TypeInt {
			a.errorAt(expression.Operator, "comparison operators expect operands of type int")
		}
		if rightType != TypeUnknown && rightType != TypeInt {
			a.errorAt(expression.Operator, "comparison operators expect operands of type int")
		}
		return TypeBool
	case TokenEqEq, TokenNeq:
		if leftType != TypeUnknown && rightType != TypeUnknown && leftType != rightType {
			a.errorAt(expression.Operator, "equality operators require operands of the same type")
		}
		return TypeBool
	default:
		return TypeUnknown
	}
}

func literalType(token Token) ValueType {
	switch token.Type {
	case TokenNumber:
		return TypeInt
	case TokenString:
		return TypeString
	case TokenTrue, TokenFalse:
		return TypeBool
	default:
		return TypeUnknown
	}
}

func expressionToken(expression Expr) Token {
	switch e := expression.(type) {
	case LiteralExpr:
		return e.Token
	case VariableExpr:
		return e.Name
	case UnaryExpr:
		return e.Operator
	case BinaryExpr:
		return e.Operator
	case AssignExpr:
		return e.Name
	case GroupingExpr:
		return expressionToken(e.Expression)
	default:
		return Token{}
	}
}
