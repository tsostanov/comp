package main

import (
	"comp/internal/ast"
	"comp/internal/executor"
	"comp/internal/lexer"
	"comp/internal/parser"
	"comp/internal/semantic"
	"fmt"
	"os"
)

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	input, err := readInput(options.filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lex := lexer.NewLexer(input)
	tokens, err := lex.Tokenize()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	parse := parser.NewParser(tokens)
	statements, err := parse.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	analyzer := semantic.NewSemanticAnalyzer()
	analyzer.Analyze(statements)
	for _, diagnostic := range analyzer.Diagnostics() {
		fmt.Fprintln(os.Stderr, diagnostic)
	}
	if analyzer.HasErrors() {
		os.Exit(1)
	}

	if options.printAST {
		printer := ast.NewAstPrinter()
		fmt.Print(printer.Print(statements))
		return
	}

	run := executor.NewExecutor(os.Stdout)
	if err := run.Execute(statements); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type cliOptions struct {
	filePath string
	printAST bool
}

func parseOptions(args []string) (cliOptions, error) {
	var options cliOptions
	for _, arg := range args {
		switch arg {
		case "--ast":
			options.printAST = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return cliOptions{}, fmt.Errorf("unknown option: %s", arg)
			}
			if options.filePath != "" {
				return cliOptions{}, fmt.Errorf("multiple input files are not supported")
			}
			options.filePath = arg
		}
	}
	return options, nil
}

func readInput(filePath string) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "var x: int = 123; print x + 5;", nil
}
