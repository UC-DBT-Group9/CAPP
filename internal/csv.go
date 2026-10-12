package kv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

type CSVOpts struct {
	HasHeader bool
	NullTok   *string
}

type CSVObj struct {
	Data   []byte
	Schema Schema
	Opts   CSVOpts
}

func NewCSVObj(data []byte, schema Schema, opts CSVOpts) CSVObj {
	return CSVObj{Data: data, Schema: schema, Opts: opts}
}

func CSVObjFromFile(path string, schema Schema, opts CSVOpts) (*CSVObj, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %s. err: %w", path, err)
	}

	csvobj := NewCSVObj(data, schema, opts)
	return &csvobj, nil
}

func (obj CSVObj) Parse() ([]Tuple, error) {
	if len(obj.Schema.Attributes) == 0 {
		return nil, fmt.Errorf("CSV schema must have at least one column")
	}
	for _, attr := range obj.Schema.Attributes {
		switch attr.DataType {
		case DataTypeString, DataTypeBool, DataTypeInt, DataTypeFloat:
		default:
			return nil, fmt.Errorf("unsupported CSV type %s for column %s", attr.DataType, attr.Name)
		}
	}

	reader := csv.NewReader(bytes.NewReader(obj.Data))
	reader.FieldsPerRecord = len(obj.Schema.Attributes)
	if obj.Opts.HasHeader {
		header, err := reader.Read()
		if err != nil {
			return nil, fmt.Errorf("read CSV header: %w", err)
		}
		for i, attr := range obj.Schema.Attributes {
			if header[i] != attr.Name {
				return nil, fmt.Errorf("CSV header column %d: expected %q, got %q", i+1, attr.Name, header[i])
			}
		}
	}

	var tuples []Tuple
	for row := 1; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			return tuples, nil
		}
		if err != nil {
			return nil, fmt.Errorf("CSV data row %d: %w", row, err)
		}
		tuple := Tuple{Values: make([]Value, len(record))}
		for i, text := range record {
			attr := obj.Schema.Attributes[i]
			if obj.Opts.NullTok != nil && text == *obj.Opts.NullTok {
				tuple.Values[i] = NullValue()
				continue
			}
			tuple.Values[i], err = parseCSVValue(text, attr.DataType)
			if err != nil {
				return nil, fmt.Errorf("CSV data row %d, column %s: %w", row, attr.Name, err)
			}
		}
		if err := obj.Schema.Validate(&tuple); err != nil {
			return nil, fmt.Errorf("CSV data row %d: %w", row, err)
		}
		tuples = append(tuples, tuple)
	}
}

func parseCSVValue(text string, dt DataType) (Value, error) {
	switch dt {
	case DataTypeString:
		return StringValue(text), nil
	case DataTypeBool:
		v, err := strconv.ParseBool(text)
		return BoolValue(v), err
	case DataTypeInt:
		v, err := strconv.ParseInt(text, 10, 64)
		return IntValue(v), err
	case DataTypeFloat:
		v, err := strconv.ParseFloat(text, 64)
		return FloatValue(v), err
	default:
		return Value{}, fmt.Errorf("unsupported CSV type %s", dt)
	}
}
