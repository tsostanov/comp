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

func (e *SemanticEnvironment) DefineVariable(name Token) (*VariableInfo, bool) {
	if _, exists := e.variables[name.Value]; exists {
		return nil, false
	}

	variable := &VariableInfo{
		Name:       name.Value,
		DeclaredAt: name,
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
		if s.Initializer != nil {
			a.VisitExpression(s.Initializer)
		}

		variable, ok := a.environment.DefineVariable(s.Name)
		if !ok {
			a.errorAt(s.Name, "variable "+s.Name.Value+" is already declared in this scope")
			return
		}
		a.variables = append(a.variables, variable)

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
		a.VisitExpression(s.Condition)
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
		a.VisitExpression(s.Condition)
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

func (a *SemanticAnalyzer) VisitExpression(expression Expr) {
	switch e := expression.(type) {
	case LiteralExpr:
		return
	case VariableExpr:
		variable := a.environment.ResolveVariable(e.Name.Value)
		if variable == nil || !variable.Flags.Defined {
			a.errorAt(e.Name, "use of undeclared variable "+e.Name.Value)
			return
		}

		variable.Flags.Used = true
		if !variable.Flags.Initialized {
			a.errorAt(e.Name, "variable "+e.Name.Value+" is used before initialization")
		}
	case AssignExpr:
		a.VisitExpression(e.Value)

		variable := a.environment.ResolveVariable(e.Name.Value)
		if variable == nil || !variable.Flags.Defined {
			a.errorAt(e.Name, "assignment to undeclared variable "+e.Name.Value)
			return
		}

		variable.Flags.Initialized = true
	case BinaryExpr:
		a.VisitExpression(e.Left)
		a.VisitExpression(e.Right)
	case UnaryExpr:
		a.VisitExpression(e.Right)
	case GroupingExpr:
		a.VisitExpression(e.Expression)
	}
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
