# comp

Small educational compiler front-end in Go. It includes:

- a lexer;
- a parser that builds an AST;
- a text AST printer;
- a semantic analyzer with scope, initialization, and type checks.

## Language Features

The language supports:

- variable declarations: `var x: int;`, `var x: int = 10;`, `var x = 10;`
- assignment: `x = 20;`
- output: `print x;`
- blocks: `{ ... }`
- conditions: `if (...) ... else ...`
- loops: `while (...) ...`
- literals: integers, strings, booleans (`true`, `false`)
- operators: `+`, `-`, `*`, `/`, `!`, `==`, `!=`, `<`, `<=`, `>`, `>=`, `and`, `or`, `&&`, `||`

## Type System

The language is statically and strictly typed.

Available types:

- `int`
- `bool`
- `string`

Rules:

- a variable must have an explicit type annotation or an initializer
- if the type is omitted, it is inferred from the initializer
- assignments must match the declared or inferred type
- `if` and `while` conditions must be `bool`
- arithmetic operators work with `int`
- logical operators work with `bool`
- `+` supports `int + int` and `string + string`
- equality operators require both operands to have the same type

## Semantic Checks

The analyzer reports:

- use of an undeclared variable
- assignment to an undeclared variable
- use before initialization
- redeclaration in the same scope
- unused variables
- type mismatches in declarations and assignments
- invalid operand types in expressions

## Run

Requirements: Go 1.25+.

Run with a file:

```bash
go run . program.txt
```

If no file path is provided, the built-in sample is used:

```txt
var x: int = 123; print x + 5;
```

## Example Program

`program.txt`:

```txt
var x: int;
if (true) {
    x = 10;
} else {
    x = 20;
}
print x;
```

## Tests

```bash
go test ./...
```
