package kv

import (
	"cmp"
	"fmt"
	"strings"
)

type Op int

const (
	OpEq Op = iota
	OpGT
	OpLT
	OpAnd
	OpOr
	OpLiteral
	OpField
	OpAttrNum
)

type Expr struct {
	Op    Op
	Value Value
	Index uint
	Name  string
	Left  *Expr
	Right *Expr
}
type NamedExpr struct {
	Name string
	Expr *Expr
}

func Literal(v Value) Expr { return Expr{Op: OpLiteral, Value: v} }

func Field(name string) Expr              { return Expr{Op: OpField, Name: name} }
func Binary(op Op, left, right Expr) Expr { return Expr{Op: op, Left: &left, Right: &right} }
func AttrNum(index uint) Expr             { return Expr{Op: OpAttrNum, Index: index} }

func (e Expr) Resolve(schema Schema) (Expr, error) {
	if e.Op == OpField {
		if i, found := schema.AttributeIndex(e.Name); found {
			return AttrNum(uint(i)), nil
		}
		return Expr{}, fmt.Errorf("unknown field %q", e.Name)
	}
	if e.Op == OpAttrNum && e.Index >= uint(len(schema.Attributes)) {
		return Expr{}, fmt.Errorf("attribute index %d out of range for schema with %d attributes", e.Index, len(schema.Attributes))
	}
	if e.Left != nil {
		left, err := e.Left.Resolve(schema)
		if err != nil {
			return Expr{}, err
		}
		e.Left = &left
	}
	if e.Right != nil {
		right, err := e.Right.Resolve(schema)
		if err != nil {
			return Expr{}, err
		}
		e.Right = &right
	}
	return e, nil
}

func (e Expr) Eval(tuple Tuple) (Value, error) {
	switch e.Op {
	case OpLiteral:
		return e.Value, nil
	case OpField:
		return Value{}, fmt.Errorf("unresolved field %q: resolve expression against a schema before evaluation", e.Name)
	case OpEq, OpGT, OpLT, OpAnd, OpOr:
		if e.Left == nil || e.Right == nil {
			return Value{}, fmt.Errorf("op requires two operands")
		}
	case OpAttrNum:
		if e.Index >= uint(len(tuple.Values)) {
			return Value{}, fmt.Errorf("attribute index %d out of range for tuple with %d values", e.Index, len(tuple.Values))
		}
		return tuple.Values[e.Index], nil
	default:
		return Value{}, fmt.Errorf("unknown op %d", e.Op)
	}

	left, err := e.Left.Eval(tuple)
	if err != nil {
		return Value{}, err
	}
	if e.Op == OpAnd || e.Op == OpOr {
		if left.DataType != DataTypeBool {
			return Value{}, fmt.Errorf("logical op must be bool, got %s", left.DataType)
		}
		if (e.Op == OpAnd && !left.Bool) || (e.Op == OpOr && left.Bool) {
			return left, nil
		}

		right, err := e.Right.Eval(tuple)
		if err != nil {
			return Value{}, err
		}
		if right.DataType != DataTypeBool {
			return Value{}, fmt.Errorf("logical op must be bool, got %s", right.DataType)
		}
		return right, nil
	}

	right, err := e.Right.Eval(tuple)
	if err != nil {
		return Value{}, err
	}

	if e.Op == OpEq {
		return BoolValue(areEqual(left, right)), nil
	}
	if left.DataType == DataTypeNull || right.DataType == DataTypeNull {
		return BoolValue(false), nil
	}

	comp, err := CompareValues(left, right)
	if err != nil {
		return Value{}, err
	}
	return BoolValue((e.Op == OpGT && comp > 0) || (e.Op == OpLT && comp < 0)), nil
}

func isNumeric(v Value) bool { return v.DataType == DataTypeInt || v.DataType == DataTypeFloat }

func toFloat(v Value) float64 {
	if v.DataType == DataTypeInt {
		return float64(v.Int)
	}
	return v.Float
}

func CompareValues(a, b Value) (int, error) {
	if a.DataType == DataTypeInt && b.DataType == DataTypeInt {
		return cmp.Compare(a.Int, b.Int), nil
	}
	if isNumeric(a) && isNumeric(b) {
		return cmp.Compare(toFloat(a), toFloat(b)), nil
	}
	if a.DataType == DataTypeString && b.DataType == DataTypeString {
		return strings.Compare(a.String, b.String), nil
	}
	return 0, fmt.Errorf("cannot order %s and %s", a.DataType, b.DataType)
}

func areEqual(a, b Value) bool {
	if a.DataType == DataTypeInt && b.DataType == DataTypeInt {
		return a.Int == b.Int
	}
	if isNumeric(a) && isNumeric(b) {
		return toFloat(a) == toFloat(b)
	}
	if a.DataType != b.DataType {
		return false
	}
	switch a.DataType {
	case DataTypeNull:
		return true
	case DataTypeBool:
		return a.Bool == b.Bool
	case DataTypeString:
		return a.String == b.String
	default:
	}
	return false
}
