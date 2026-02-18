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

	for _, tok := range tokens {
		fmt.Println(tok)
	}
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
