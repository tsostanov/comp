package executor

import (
	"comp/internal/lexer"
	"comp/internal/parser"
	"comp/internal/semantic"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutorRunsMainExampleProgram(t *testing.T) {
	source := readExampleFile(t, "program.txt")

	output, err := executeSource(t, source)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	expected := "large\n57\n0\n1\n2\n"
	if output != expected {
		t.Fatalf("expected output %q, got %q", expected, output)
	}
}

func readExampleFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "examples", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example %s failed: %v", name, err)
	}
	return string(data)
}

func executeSource(t *testing.T, source string) (string, error) {
	t.Helper()

	lex := lexer.NewLexer(source)
	tokens, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("tokenize failed: %v", err)
	}

	parse := parser.NewParser(tokens)
	statements, err := parse.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	analyzer := semantic.NewSemanticAnalyzer()
	diagnostics := analyzer.Analyze(statements)
	if analyzer.HasErrors() {
		t.Fatalf("unexpected semantic errors: %#v", diagnostics)
	}

	var output strings.Builder
	executor := NewExecutor(&output)
	err = executor.Execute(statements)
	return output.String(), err
}
