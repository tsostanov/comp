package main

import (
	"fmt"
	"os"
)

func main() {
	input, err := readInput()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	parser := NewParser(tokens)
	statements, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	analyzer := NewSemanticAnalyzer()
	analyzer.Analyze(statements)
	for _, diagnostic := range analyzer.Diagnostics() {
		fmt.Fprintln(os.Stderr, diagnostic)
	}
	if analyzer.HasErrors() {
		os.Exit(1)
	}

	printer := NewAstPrinter()
	fmt.Print(printer.Print(statements))
}

func readInput() (string, error) {
	if len(os.Args) > 1 {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "var x = 123; print x + 5;", nil
}
