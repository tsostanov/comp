package main

import (
	"strings"
	"testing"
)

func TestSemanticEnvironmentDefineAndResolveVariable(t *testing.T) {
	global := NewSemanticEnvironment(nil)
	token := Token{Value: "x", Line: 1, Column: 5}

	variable, ok := global.DefineVariable(token)
	if !ok {
		t.Fatalf("expected variable definition to succeed")
	}
	if variable == nil {
		t.Fatalf("expected variable info to be returned")
	}
	if !variable.Flags.Defined {
		t.Fatalf("expected variable to be marked as defined")
	}

	child := NewSemanticEnvironment(global)
	resolved := child.ResolveVariable("x")
	if resolved != variable {
		t.Fatalf("expected child environment to resolve parent variable")
	}
}

func TestSemanticEnvironmentRejectsSameScopeRedeclaration(t *testing.T) {
	environment := NewSemanticEnvironment(nil)
	token := Token{Value: "x", Line: 1, Column: 5}

	if _, ok := environment.DefineVariable(token); !ok {
		t.Fatalf("expected first declaration to succeed")
	}
	if _, ok := environment.DefineVariable(token); ok {
		t.Fatalf("expected second declaration in same scope to fail")
	}
}

func TestSemanticAnalyzerUseBeforeInitialization(t *testing.T) {
	diagnostics := analyzeSource(t, "var x; print x;")

	assertHasDiagnostic(t, diagnostics, SeverityError, "used before initialization")
}

func TestSemanticAnalyzerUndeclaredVariable(t *testing.T) {
	diagnostics := analyzeSource(t, "print x; x = 1;")

	assertHasDiagnostic(t, diagnostics, SeverityError, "use of undeclared variable x")
	assertHasDiagnostic(t, diagnostics, SeverityError, "assignment to undeclared variable x")
}

func TestSemanticAnalyzerInitializerMarksVariableInitialized(t *testing.T) {
	diagnostics := analyzeSource(t, "var x = 1; print x;")

	assertNoDiagnostic(t, diagnostics, SeverityError, "used before initialization")
}

func TestSemanticAnalyzerAssignmentMarksVariableInitialized(t *testing.T) {
	diagnostics := analyzeSource(t, "var x; x = 1; print x;")

	assertNoDiagnostic(t, diagnostics, SeverityError, "used before initialization")
}

func TestSemanticAnalyzerIfElsePropagatesInitialization(t *testing.T) {
	diagnostics := analyzeSource(t, `
var x;
if (1) {
	x = 1;
} else {
	x = 2;
}
print x;
`)

	assertNoDiagnostic(t, diagnostics, SeverityError, "used before initialization")
}

func TestSemanticAnalyzerIfWithoutElseDoesNotGuaranteeInitialization(t *testing.T) {
	diagnostics := analyzeSource(t, `
var x;
if (1) {
	x = 1;
}
print x;
`)

	assertHasDiagnostic(t, diagnostics, SeverityError, "used before initialization")
}

func TestSemanticAnalyzerWhileDoesNotGuaranteeInitialization(t *testing.T) {
	diagnostics := analyzeSource(t, `
var x;
while (0) {
	x = 1;
}
print x;
`)

	assertHasDiagnostic(t, diagnostics, SeverityError, "used before initialization")
}

func TestSemanticAnalyzerUnusedVariableWarning(t *testing.T) {
	diagnostics := analyzeSource(t, "var x = 1;")

	assertHasDiagnostic(t, diagnostics, SeverityWarning, "declared but never used")
}

func TestSemanticAnalyzerBlockScopeRedeclarationAllowed(t *testing.T) {
	diagnostics := analyzeSource(t, `
var x = 1;
{
	var x = 2;
	print x;
}
print x;
`)

	assertNoDiagnostic(t, diagnostics, SeverityError, "already declared in this scope")
}

func TestSemanticAnalyzerSameScopeRedeclarationRejected(t *testing.T) {
	diagnostics := analyzeSource(t, "var x = 1; var x = 2;")

	assertHasDiagnostic(t, diagnostics, SeverityError, "already declared in this scope")
}

func analyzeSource(t *testing.T, source string) []SemanticDiagnostic {
	t.Helper()

	lexer := NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize failed: %v", err)
	}

	parser := NewParser(tokens)
	statements, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := NewSemanticAnalyzer()
	return analyzer.Analyze(statements)
}

func assertHasDiagnostic(t *testing.T, diagnostics []SemanticDiagnostic, severity DiagnosticSeverity, fragment string) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.Message, fragment) {
			return
		}
	}

	t.Fatalf("expected %s containing %q, got %#v", severity, fragment, diagnostics)
}

func assertNoDiagnostic(t *testing.T, diagnostics []SemanticDiagnostic, severity DiagnosticSeverity, fragment string) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.Message, fragment) {
			t.Fatalf("unexpected %s containing %q: %#v", severity, fragment, diagnostic)
		}
	}
}
