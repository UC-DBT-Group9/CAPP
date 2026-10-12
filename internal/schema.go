package kv

import (
	"fmt"
)

type Tuple struct {
	Values []Value
}

type Attribute struct {
	DataType DataType
	Name     string
	Required bool
}

type Schema struct {
	Name       string
	Attributes []Attribute
}

func NewAttribute(dt DataType, name string, required bool) (*Attribute, error) {
	return &Attribute{DataType: dt, Name: name, Required: required}, nil
}

func NewSchema(name string, attrs []Attribute) (*Schema, error) {
	for _, attr := range attrs {
		if attr.DataType == DataTypeNull {
			return nil, fmt.Errorf("DataType in schema cannot be Null. If you wish to make %s nullable, use Required: false", attr.Name)
		}
	}
	return &Schema{Name: name, Attributes: attrs}, nil
}

func (schema *Schema) Validate(tuple *Tuple) error {
	if len(tuple.Values) != len(schema.Attributes) {
		return fmt.Errorf("tuple has %d values, schema %s expects %d", len(tuple.Values), schema.Name, len(schema.Attributes))
	}
	for i, val := range tuple.Values {
		if val.DataType == DataTypeNull {
			if schema.Attributes[i].Required {
				return fmt.Errorf("tuple missing required attribute %s", schema.Attributes[i].Name)
			}
			continue
		}
		if val.DataType != schema.Attributes[i].DataType {
			return fmt.Errorf("tuple does not match schema. expected %s at position %d, found %s", schema.Attributes[i].DataType.String(), i, val.DataType.String())
		}
	}
	return nil
}
