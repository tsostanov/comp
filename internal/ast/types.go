package ast

import tok "comp/internal/token"

type ValueType int

const (
	TypeUnknown ValueType = iota
	TypeInt
	TypeBool
	TypeString
)

func (t ValueType) String() string {
	switch t {
	case TypeInt:
		return "int"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	default:
		return "unknown"
	}
}

type TypeAnnotation struct {
	Name tok.Token
	Kind ValueType
}
