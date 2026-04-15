package executor

import (
	"comp/internal/ast"
	tok "comp/internal/token"
	"fmt"
	"io"
	"strconv"
)

type RuntimeError struct {
	Message string
	Line    int
	Column  int
}

func (e RuntimeError) Error() string {
	return fmt.Sprintf("runtime error at %d:%d: %s", e.Line, e.Column, e.Message)
}

type Value struct {
	Type ast.ValueType
	Data any
}

func (v Value) String() string {
	switch v.Type {
	case ast.TypeInt:
		return strconv.Itoa(v.Data.(int))
	case ast.TypeBool:
		if v.Data.(bool) {
			return "true"
		}
		return "false"
	case ast.TypeString:
		return v.Data.(string)
	default:
		return "<unknown>"
	}
}

type VariableSlot struct {
	Type        ast.ValueType
	Value       Value
	Initialized bool
}

type Environment struct {
	parent    *Environment
	variables map[string]*VariableSlot
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		parent:    parent,
		variables: make(map[string]*VariableSlot),
	}
}

func (e *Environment) DefineVariable(name string, variableType ast.ValueType, value Value, initialized bool) bool {
	if _, exists := e.variables[name]; exists {
		return false
	}

	e.variables[name] = &VariableSlot{
		Type:        variableType,
		Value:       value,
		Initialized: initialized,
	}
	return true
}

func (e *Environment) ResolveVariable(name string) *VariableSlot {
	for current := e; current != nil; current = current.parent {
		if variable, ok := current.variables[name]; ok {
			return variable
		}
	}
	return nil
}

type Executor struct {
	environment *Environment
	output      io.Writer
}

func NewExecutor(output io.Writer) *Executor {
	return &Executor{
		environment: NewEnvironment(nil),
		output:      output,
	}
}

func (e *Executor) Execute(statements []ast.Stmt) error {
	for _, statement := range statements {
		if err := e.executeStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) executeStatement(statement ast.Stmt) error {
	switch s := statement.(type) {
	case ast.VarStmt:
		return e.executeVarStatement(s)
	case ast.PrintStmt:
		value, err := e.evaluateExpression(s.Expression)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(e.output, value.String())
		return err
	case ast.ExprStmt:
		_, err := e.evaluateExpression(s.Expression)
		return err
	case ast.BlockStmt:
		previous := e.environment
		e.environment = NewEnvironment(previous)
		defer func() {
			e.environment = previous
		}()

		for _, nested := range s.Statements {
			if err := e.executeStatement(nested); err != nil {
				return err
			}
		}
		return nil
	case ast.IfStmt:
		condition, err := e.evaluateExpression(s.Condition)
		if err != nil {
			return err
		}
		conditionValue, err := e.expectBool(condition, expressionToken(s.Condition), "if condition must be bool")
		if err != nil {
			return err
		}
		if conditionValue {
			return e.executeStatement(s.ThenBranch)
		}
		if s.ElseBranch != nil {
			return e.executeStatement(s.ElseBranch)
		}
		return nil
	case ast.WhileStmt:
		for {
			condition, err := e.evaluateExpression(s.Condition)
			if err != nil {
				return err
			}
			conditionValue, err := e.expectBool(condition, expressionToken(s.Condition), "while condition must be bool")
			if err != nil {
				return err
			}
			if !conditionValue {
				return nil
			}
			if err := e.executeStatement(s.Body); err != nil {
				return err
			}
		}
	default:
		return nil
	}
}

func (e *Executor) executeVarStatement(statement ast.VarStmt) error {
	variableType := ast.TypeUnknown
	if statement.DeclaredType != nil {
		variableType = statement.DeclaredType.Kind
	}

	var value Value
	initialized := false
	if statement.Initializer != nil {
		initializerValue, err := e.evaluateExpression(statement.Initializer)
		if err != nil {
			return err
		}
		if variableType == ast.TypeUnknown {
			variableType = initializerValue.Type
		}
		if variableType != initializerValue.Type {
			return runtimeError(statement.Name, "cannot initialize variable "+statement.Name.Value+" of type "+variableType.String()+" with value of type "+initializerValue.Type.String())
		}
		value = initializerValue
		initialized = true
	} else {
		if variableType == ast.TypeUnknown {
			return runtimeError(statement.Name, "variable "+statement.Name.Value+" requires an explicit type or initializer")
		}
		value = zeroValue(variableType)
	}

	if !e.environment.DefineVariable(statement.Name.Value, variableType, value, initialized) {
		return runtimeError(statement.Name, "variable "+statement.Name.Value+" is already declared in this scope")
	}
	return nil
}

func (e *Executor) evaluateExpression(expression ast.Expr) (Value, error) {
	switch expr := expression.(type) {
	case ast.LiteralExpr:
		return literalValue(expr.Token)
	case ast.VariableExpr:
		variable := e.environment.ResolveVariable(expr.Name.Value)
		if variable == nil {
			return Value{}, runtimeError(expr.Name, "use of undeclared variable "+expr.Name.Value)
		}
		if !variable.Initialized {
			return Value{}, runtimeError(expr.Name, "variable "+expr.Name.Value+" is used before initialization")
		}
		return variable.Value, nil
	case ast.AssignExpr:
		value, err := e.evaluateExpression(expr.Value)
		if err != nil {
			return Value{}, err
		}

		variable := e.environment.ResolveVariable(expr.Name.Value)
		if variable == nil {
			return Value{}, runtimeError(expr.Name, "assignment to undeclared variable "+expr.Name.Value)
		}
		if variable.Type != ast.TypeUnknown && variable.Type != value.Type {
			return Value{}, runtimeError(expr.Name, "cannot assign value of type "+value.Type.String()+" to variable "+expr.Name.Value+" of type "+variable.Type.String())
		}

		variable.Value = value
		variable.Initialized = true
		return value, nil
	case ast.GroupingExpr:
		return e.evaluateExpression(expr.Expression)
	case ast.UnaryExpr:
		right, err := e.evaluateExpression(expr.Right)
		if err != nil {
			return Value{}, err
		}
		return e.evaluateUnary(expr.Operator, right)
	case ast.BinaryExpr:
		return e.evaluateBinary(expr)
	default:
		return Value{}, RuntimeError{Message: "unsupported expression"}
	}
}

func (e *Executor) evaluateUnary(operator tok.Token, right Value) (Value, error) {
	switch operator.Type {
	case tok.TokenMinus:
		value, err := e.expectInt(right, operator, "operator - expects operand of type int")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: ast.TypeInt, Data: -value}, nil
	case tok.TokenExcl:
		value, err := e.expectBool(right, operator, "operator ! expects operand of type bool")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: ast.TypeBool, Data: !value}, nil
	default:
		return Value{}, runtimeError(operator, "unsupported unary operator "+operator.Value)
	}
}

func (e *Executor) evaluateBinary(expression ast.BinaryExpr) (Value, error) {
	switch expression.Operator.Type {
	case tok.TokenAnd:
		left, err := e.evaluateExpression(expression.Left)
		if err != nil {
			return Value{}, err
		}
		leftValue, err := e.expectBool(left, expression.Operator, "logical operators expect operands of type bool")
		if err != nil {
			return Value{}, err
		}
		if !leftValue {
			return Value{Type: ast.TypeBool, Data: false}, nil
		}
		right, err := e.evaluateExpression(expression.Right)
		if err != nil {
			return Value{}, err
		}
		rightValue, err := e.expectBool(right, expression.Operator, "logical operators expect operands of type bool")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: ast.TypeBool, Data: rightValue}, nil
	case tok.TokenOr:
		left, err := e.evaluateExpression(expression.Left)
		if err != nil {
			return Value{}, err
		}
		leftValue, err := e.expectBool(left, expression.Operator, "logical operators expect operands of type bool")
		if err != nil {
			return Value{}, err
		}
		if leftValue {
			return Value{Type: ast.TypeBool, Data: true}, nil
		}
		right, err := e.evaluateExpression(expression.Right)
		if err != nil {
			return Value{}, err
		}
		rightValue, err := e.expectBool(right, expression.Operator, "logical operators expect operands of type bool")
		if err != nil {
			return Value{}, err
		}
		return Value{Type: ast.TypeBool, Data: rightValue}, nil
	}

	left, err := e.evaluateExpression(expression.Left)
	if err != nil {
		return Value{}, err
	}
	right, err := e.evaluateExpression(expression.Right)
	if err != nil {
		return Value{}, err
	}

	switch expression.Operator.Type {
	case tok.TokenPlus:
		if left.Type == ast.TypeInt && right.Type == ast.TypeInt {
			return Value{Type: ast.TypeInt, Data: left.Data.(int) + right.Data.(int)}, nil
		}
		if left.Type == ast.TypeString && right.Type == ast.TypeString {
			return Value{Type: ast.TypeString, Data: left.Data.(string) + right.Data.(string)}, nil
		}
		return Value{}, runtimeError(expression.Operator, "operator + expects operands of type int or string")
	case tok.TokenMinus:
		return e.evaluateIntBinary(expression.Operator, left, right, func(a, b int) (int, error) {
			return a - b, nil
		})
	case tok.TokenStar:
		return e.evaluateIntBinary(expression.Operator, left, right, func(a, b int) (int, error) {
			return a * b, nil
		})
	case tok.TokenSlash:
		return e.evaluateIntBinary(expression.Operator, left, right, func(a, b int) (int, error) {
			if b == 0 {
				return 0, runtimeError(expression.Operator, "division by zero")
			}
			return a / b, nil
		})
	case tok.TokenLt:
		return e.evaluateIntComparison(expression.Operator, left, right, func(a, b int) bool { return a < b })
	case tok.TokenLtEq:
		return e.evaluateIntComparison(expression.Operator, left, right, func(a, b int) bool { return a <= b })
	case tok.TokenGt:
		return e.evaluateIntComparison(expression.Operator, left, right, func(a, b int) bool { return a > b })
	case tok.TokenGtEq:
		return e.evaluateIntComparison(expression.Operator, left, right, func(a, b int) bool { return a >= b })
	case tok.TokenEqEq:
		return Value{Type: ast.TypeBool, Data: valuesEqual(left, right)}, nil
	case tok.TokenNeq:
		return Value{Type: ast.TypeBool, Data: !valuesEqual(left, right)}, nil
	default:
		return Value{}, runtimeError(expression.Operator, "unsupported binary operator "+expression.Operator.Value)
	}
}

func (e *Executor) evaluateIntBinary(operator tok.Token, left, right Value, operation func(int, int) (int, error)) (Value, error) {
	leftValue, err := e.expectInt(left, operator, "arithmetic operators expect operands of type int")
	if err != nil {
		return Value{}, err
	}
	rightValue, err := e.expectInt(right, operator, "arithmetic operators expect operands of type int")
	if err != nil {
		return Value{}, err
	}

	result, err := operation(leftValue, rightValue)
	if err != nil {
		return Value{}, err
	}
	return Value{Type: ast.TypeInt, Data: result}, nil
}

func (e *Executor) evaluateIntComparison(operator tok.Token, left, right Value, compare func(int, int) bool) (Value, error) {
	leftValue, err := e.expectInt(left, operator, "comparison operators expect operands of type int")
	if err != nil {
		return Value{}, err
	}
	rightValue, err := e.expectInt(right, operator, "comparison operators expect operands of type int")
	if err != nil {
		return Value{}, err
	}

	return Value{Type: ast.TypeBool, Data: compare(leftValue, rightValue)}, nil
}

func (e *Executor) expectInt(value Value, token tok.Token, message string) (int, error) {
	if value.Type != ast.TypeInt {
		return 0, runtimeError(token, message)
	}
	return value.Data.(int), nil
}

func (e *Executor) expectBool(value Value, token tok.Token, message string) (bool, error) {
	if value.Type != ast.TypeBool {
		return false, runtimeError(token, message)
	}
	return value.Data.(bool), nil
}

func literalValue(token tok.Token) (Value, error) {
	switch token.Type {
	case tok.TokenNumber:
		value, err := strconv.Atoi(token.Value)
		if err != nil {
			return Value{}, runtimeError(token, "invalid integer literal "+token.Value)
		}
		return Value{Type: ast.TypeInt, Data: value}, nil
	case tok.TokenString:
		return Value{Type: ast.TypeString, Data: token.Value}, nil
	case tok.TokenTrue:
		return Value{Type: ast.TypeBool, Data: true}, nil
	case tok.TokenFalse:
		return Value{Type: ast.TypeBool, Data: false}, nil
	default:
		return Value{}, runtimeError(token, "unsupported literal "+token.Value)
	}
}

func zeroValue(valueType ast.ValueType) Value {
	switch valueType {
	case ast.TypeInt:
		return Value{Type: ast.TypeInt, Data: 0}
	case ast.TypeBool:
		return Value{Type: ast.TypeBool, Data: false}
	case ast.TypeString:
		return Value{Type: ast.TypeString, Data: ""}
	default:
		return Value{Type: ast.TypeUnknown, Data: nil}
	}
}

func valuesEqual(left, right Value) bool {
	if left.Type != right.Type {
		return false
	}

	switch left.Type {
	case ast.TypeInt:
		return left.Data.(int) == right.Data.(int)
	case ast.TypeBool:
		return left.Data.(bool) == right.Data.(bool)
	case ast.TypeString:
		return left.Data.(string) == right.Data.(string)
	default:
		return left.Data == right.Data
	}
}

func runtimeError(token tok.Token, message string) error {
	return RuntimeError{
		Message: message,
		Line:    token.Line,
		Column:  token.Column,
	}
}

func expressionToken(expression ast.Expr) tok.Token {
	switch expr := expression.(type) {
	case ast.LiteralExpr:
		return expr.Token
	case ast.VariableExpr:
		return expr.Name
	case ast.UnaryExpr:
		return expr.Operator
	case ast.BinaryExpr:
		return expr.Operator
	case ast.AssignExpr:
		return expr.Name
	case ast.GroupingExpr:
		return expressionToken(expr.Expression)
	default:
		return tok.Token{}
	}
}
