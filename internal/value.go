package kv

import "fmt"

type DataType int

const (
	DataTypeNull DataType = iota
	DataTypeBool
	DataTypeInt
	DataTypeFloat
	DataTypeString
	DataTypeBytes
	DataTypeArray
	DataTypeObject
)

func (dt DataType) String() string {
	names := [...]string{"null", "bool", "int", "float", "string", "bytes", "array", "object"}
	if int(dt) >= len(names) {
		return fmt.Sprintf("DataType(%d)", dt)
	}
	return names[dt]
}

type Value struct {
	DataType DataType
	Bool     bool
	Int      int64
	Float    float64
	String   string
	Bytes    []byte
	Array    []Value
	Object   map[string]Value
}

func NullValue() Value                     { return Value{DataType: DataTypeNull} }
func BoolValue(v bool) Value               { return Value{DataType: DataTypeBool, Bool: v} }
func IntValue(v int64) Value               { return Value{DataType: DataTypeInt, Int: v} }
func FloatValue(v float64) Value           { return Value{DataType: DataTypeFloat, Float: v} }
func StringValue(v string) Value           { return Value{DataType: DataTypeString, String: v} }
func BytesValue(v []byte) Value            { return Value{DataType: DataTypeBytes, Bytes: v} }
func ArrayValue(v []Value) Value           { return Value{DataType: DataTypeArray, Array: v} }
func ObjectValue(v map[string]Value) Value { return Value{DataType: DataTypeObject, Object: v} }
