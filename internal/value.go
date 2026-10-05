package kv

import "fmt"

type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindBytes
	KindArray
	KindObject
)

func (k Kind) String() string {
	names := [...]string{"null", "bool", "int", "float", "string", "bytes", "array", "object"}
	if int(k) >= len(names) {
		return fmt.Sprintf("Kind(%d)", k)
	}
	return names[k]
}

type Value struct {
	Kind   Kind
	Bool   bool
	Int    int64
	Float  float64
	String string
	Bytes  []byte
	Array  []Value
	Object map[string]Value
}

func NullValue() Value                     { return Value{Kind: KindNull} }
func BoolValue(v bool) Value               { return Value{Kind: KindBool, Bool: v} }
func IntValue(v int64) Value               { return Value{Kind: KindInt, Int: v} }
func FloatValue(v float64) Value           { return Value{Kind: KindFloat, Float: v} }
func StringValue(v string) Value           { return Value{Kind: KindString, String: v} }
func BytesValue(v []byte) Value            { return Value{Kind: KindBytes, Bytes: v} }
func ArrayValue(v []Value) Value           { return Value{Kind: KindArray, Array: v} }
func ObjectValue(v map[string]Value) Value { return Value{Kind: KindObject, Object: v} }
