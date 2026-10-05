package kv

import (
	"bytes"
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
	OpRoot
)

type Expr struct {
	Op    Op
	Value Value
	Path  []string
	Left  *Expr
	Right *Expr
}

func Literal(v Value) Expr                { return Expr{Op: OpLiteral, Value: v} }
func Field(path ...string) Expr           { return Expr{Op: OpField, Path: path} }
func Root() Expr                          { return Expr{Op: OpRoot} }
func Binary(op Op, left, right Expr) Expr { return Expr{Op: op, Left: &left, Right: &right} }

func (e Expr) Eval(record Value) (Value, error) {
	switch e.Op {
	case OpLiteral:
		return e.Value, nil
	case OpRoot:
		return record, nil
	case OpField:
		v := record
		for _, name := range e.Path {
			if v.Kind != KindObject {
				return NullValue(), nil
			}
			var exists bool
			v, exists = v.Object[name]
			if !exists {
				return NullValue(), nil
			}
		}
		return v, nil
	case OpEq, OpGT, OpLT, OpAnd, OpOr:
		if e.Left == nil || e.Right == nil {
			return Value{}, fmt.Errorf("op requires two operands")
		}
	default:
		return Value{}, fmt.Errorf("unknown op %d", e.Op)
	}

	left, err := e.Left.Eval(record)
	if err != nil {
		return Value{}, err
	}
	if e.Op == OpAnd || e.Op == OpOr {
		if left.Kind != KindBool {
			return Value{}, fmt.Errorf("logical op must be bool, got %s", left.Kind)
		}
		if (e.Op == OpAnd && !left.Bool) || (e.Op == OpOr && left.Bool) {
			return left, nil
		}

		right, err := e.Right.Eval(record)
		if err != nil {
			return Value{}, err
		}
		if right.Kind != KindBool {
			return Value{}, fmt.Errorf("logical op must be bool, got %s", right.Kind)
		}
		return right, nil
	}

	right, err := e.Right.Eval(record)
	if err != nil {
		return Value{}, err
	}

	if e.Op == OpEq {
		return BoolValue(areEqual(left, right)), nil
	}
	if left.Kind == KindNull || right.Kind == KindNull {
		return BoolValue(false), nil
	}

	cmp, err := CompareValues(left, right)
	if err != nil {
		return Value{}, err
	}
	return BoolValue((e.Op == OpGT && cmp > 0) || (e.Op == OpLT && cmp < 0)), nil
}

func isNumeric(v Value) bool { return v.Kind == KindInt || v.Kind == KindFloat }

func toFloat(v Value) float64 {
	if v.Kind == KindInt {
		return float64(v.Int)
	}
	return v.Float
}

func CompareValues(a, b Value) (int, error) {
	if a.Kind == KindInt && b.Kind == KindInt {
		return cmp.Compare(a.Int, b.Int), nil
	}
	if isNumeric(a) && isNumeric(b) {
		return cmp.Compare(toFloat(a), toFloat(b)), nil
	}
	if a.Kind == KindString && b.Kind == KindString {
		return strings.Compare(a.String, b.String), nil
	}
	if a.Kind == KindBytes && b.Kind == KindBytes {
		return bytes.Compare(a.Bytes, b.Bytes), nil
	}
	return 0, fmt.Errorf("cannot order %s and %s", a.Kind, b.Kind)
}

func areEqual(a, b Value) bool {
	if a.Kind == KindInt && b.Kind == KindInt {
		return a.Int == b.Int
	}
	if isNumeric(a) && isNumeric(b) {
		return toFloat(a) == toFloat(b)
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindNull:
		return true
	case KindBool:
		return a.Bool == b.Bool
	case KindString:
		return a.String == b.String
	case KindBytes:
		return bytes.Equal(a.Bytes, b.Bytes)
	case KindArray:
		if len(a.Array) != len(b.Array) {
			return false
		}
		for i := range a.Array {
			if !areEqual(a.Array[i], b.Array[i]) {
				return false
			}
		}
		return true
	case KindObject:
		if len(a.Object) != len(b.Object) {
			return false
		}
		for name, item := range a.Object {
			other, exists := b.Object[name]
			if !exists || !areEqual(item, other) {
				return false
			}
		}
		return true
	}
	return false
}
