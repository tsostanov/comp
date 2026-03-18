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

type Symbol struct {
	Name       string
	DeclaredAt Token
	Flags      SymbolFlags
}

type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
}

type symbolState struct {
	defined     bool
	initialized bool
	used        bool
}

type SemanticAnalyzer struct {
	currentScope *Scope
	allSymbols   []*Symbol
	diagnostics  []SemanticDiagnostic
}

func NewSemanticAnalyzer() *SemanticAnalyzer {
	globalScope := &Scope{symbols: make(map[string]*Symbol)}
	return &SemanticAnalyzer{
		currentScope: globalScope,
	}
}

func (a *SemanticAnalyzer) Analyze(stmts []Stmt) []SemanticDiagnostic {
	for _, stmt := range stmts {
		a.analyzeStmt(stmt)
	}
	a.reportUnusedSymbols()
	return a.diagnostics
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

func (a *SemanticAnalyzer) analyzeStmt(stmt Stmt) {
	switch s := stmt.(type) {
	case VarStmt:
		symbol := a.declare(s.Name)
		if s.Initializer != nil {
			a.analyzeExpr(s.Initializer)
			if symbol != nil {
				symbol.Flags.Initialized = true
			}
		}
	case PrintStmt:
		a.analyzeExpr(s.Expression)
	case ExprStmt:
		a.analyzeExpr(s.Expression)
	case BlockStmt:
		a.beginScope()
		for _, nested := range s.Statements {
			a.analyzeStmt(nested)
		}
		a.endScope()
	case IfStmt:
		a.analyzeExpr(s.Condition)
		before := a.snapshotStates()

		a.analyzeStmt(s.ThenBranch)
		thenState := a.snapshotStates()

		if s.ElseBranch == nil {
			a.restoreStates(before)
			a.mergeStates(before, thenState, nil)
			return
		}

		a.restoreStates(before)
		a.analyzeStmt(s.ElseBranch)
		elseState := a.snapshotStates()
		a.restoreStates(before)
		a.mergeStates(before, thenState, elseState)
	case WhileStmt:
		a.analyzeExpr(s.Condition)
		before := a.snapshotStates()
		a.analyzeStmt(s.Body)
		bodyState := a.snapshotStates()
		a.restoreStates(before)

		// Loop body may never execute, so only "used" is merged back.
		for symbol, state := range before {
			body := bodyState[symbol]
			symbol.Flags.Defined = state.defined
			symbol.Flags.Initialized = state.initialized
			symbol.Flags.Used = state.used || body.used
		}
	}
}

func (a *SemanticAnalyzer) analyzeExpr(expr Expr) {
	switch e := expr.(type) {
	case LiteralExpr:
		return
	case VariableExpr:
		symbol := a.resolve(e.Name.Value)
		if symbol == nil || !symbol.Flags.Defined {
			a.errorAt(e.Name, "use of undeclared variable "+e.Name.Value)
			return
		}
		symbol.Flags.Used = true
		if !symbol.Flags.Initialized {
			a.errorAt(e.Name, "variable "+e.Name.Value+" is used before initialization")
		}
	case UnaryExpr:
		a.analyzeExpr(e.Right)
	case BinaryExpr:
		a.analyzeExpr(e.Left)
		a.analyzeExpr(e.Right)
	case AssignExpr:
		symbol := a.resolve(e.Name.Value)
		if symbol == nil || !symbol.Flags.Defined {
			a.errorAt(e.Name, "assignment to undeclared variable "+e.Name.Value)
			a.analyzeExpr(e.Value)
			return
		}
		a.analyzeExpr(e.Value)
		symbol.Flags.Initialized = true
	case GroupingExpr:
		a.analyzeExpr(e.Expression)
	}
}

func (a *SemanticAnalyzer) declare(name Token) *Symbol {
	if _, exists := a.currentScope.symbols[name.Value]; exists {
		a.errorAt(name, "variable "+name.Value+" is already declared in this scope")
		return nil
	}

	symbol := &Symbol{
		Name:       name.Value,
		DeclaredAt: name,
		Flags: SymbolFlags{
			Defined: true,
		},
	}
	a.currentScope.symbols[name.Value] = symbol
	a.allSymbols = append(a.allSymbols, symbol)
	return symbol
}

func (a *SemanticAnalyzer) resolve(name string) *Symbol {
	for scope := a.currentScope; scope != nil; scope = scope.parent {
		if symbol, ok := scope.symbols[name]; ok {
			return symbol
		}
	}
	return nil
}

func (a *SemanticAnalyzer) beginScope() {
	a.currentScope = &Scope{
		parent:  a.currentScope,
		symbols: make(map[string]*Symbol),
	}
}

func (a *SemanticAnalyzer) endScope() {
	if a.currentScope.parent != nil {
		a.currentScope = a.currentScope.parent
	}
}

func (a *SemanticAnalyzer) snapshotStates() map[*Symbol]symbolState {
	snapshot := make(map[*Symbol]symbolState, len(a.allSymbols))
	for _, symbol := range a.allSymbols {
		snapshot[symbol] = symbolState{
			defined:     symbol.Flags.Defined,
			initialized: symbol.Flags.Initialized,
			used:        symbol.Flags.Used,
		}
	}
	return snapshot
}

func (a *SemanticAnalyzer) restoreStates(snapshot map[*Symbol]symbolState) {
	for symbol, state := range snapshot {
		symbol.Flags.Defined = state.defined
		symbol.Flags.Initialized = state.initialized
		symbol.Flags.Used = state.used
	}
}

func (a *SemanticAnalyzer) mergeStates(before, left, right map[*Symbol]symbolState) {
	for symbol, state := range before {
		leftState := left[symbol]
		if right == nil {
			symbol.Flags.Defined = state.defined
			symbol.Flags.Initialized = state.initialized
			symbol.Flags.Used = state.used || leftState.used
			continue
		}

		rightState := right[symbol]
		symbol.Flags.Defined = leftState.defined && rightState.defined
		symbol.Flags.Initialized = leftState.initialized && rightState.initialized
		symbol.Flags.Used = leftState.used || rightState.used
	}
}

func (a *SemanticAnalyzer) reportUnusedSymbols() {
	for _, symbol := range a.allSymbols {
		if !symbol.Flags.Defined || symbol.Flags.Used {
			continue
		}
		a.diagnostics = append(a.diagnostics, SemanticDiagnostic{
			Severity: SeverityWarning,
			Message:  "variable " + symbol.Name + " is declared but never used",
			Line:     symbol.DeclaredAt.Line,
			Column:   symbol.DeclaredAt.Column,
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
