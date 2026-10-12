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

type CSVIterator struct {
	obj    *CSVObj
	reader *csv.Reader
	row    int
	err    error
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

func (obj *CSVObj) Iterator() *CSVIterator {
	return &CSVIterator{obj: obj}
}

func (iter *CSVIterator) initReader() error {
	obj := iter.obj
	if len(obj.Schema.Attributes) == 0 {
		return fmt.Errorf("CSV schema must have at least one column")
	}
	for _, attr := range obj.Schema.Attributes {
		switch attr.DataType {
		case DataTypeString, DataTypeBool, DataTypeInt, DataTypeFloat:
		default:
			return fmt.Errorf("unsupported CSV type %s for column %s", attr.DataType, attr.Name)
		}
	}

	iter.reader = csv.NewReader(bytes.NewReader(obj.Data))
	iter.reader.FieldsPerRecord = len(obj.Schema.Attributes)
	if obj.Opts.HasHeader {
		header, err := iter.reader.Read()
		if err != nil {
			return fmt.Errorf("read CSV header: %w", err)
		}
		for i, attr := range obj.Schema.Attributes {
			if header[i] != attr.Name {
				return fmt.Errorf("CSV header column %d: expected %q, got %q", i+1, attr.Name, header[i])
			}
		}
	}

	return nil
}

func (iter *CSVIterator) Next() (*Tuple, error) {
	if iter == nil {
		return nil, fmt.Errorf("CSV iterator must not be nil")
	}
	if iter.err != nil {
		return nil, iter.err
	}
	if iter.obj == nil {
		iter.err = fmt.Errorf("CSV iterator must have a csv obj")
		return nil, iter.err
	}
	if iter.reader == nil {
		iter.err = iter.initReader()
		if iter.err != nil {
			return nil, iter.err
		}
	}

	obj := iter.obj
	iter.row++
	record, err := iter.reader.Read()
	if err != nil {
		iter.err = err
		if err != io.EOF {
			iter.err = fmt.Errorf("CSV data row %d: %w", iter.row, err)
		}
		return nil, iter.err
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
			iter.err = fmt.Errorf("CSV data row %d, column %s: %w", iter.row, attr.Name, err)
			return nil, iter.err
		}
	}

	if err := obj.Schema.Validate(&tuple); err != nil {
		iter.err = fmt.Errorf("CSV data row %d: %w", iter.row, err)
		return nil, iter.err
	}
	return &tuple, nil
}

func (obj CSVObj) Parse() ([]Tuple, error) {
	iter := obj.Iterator()
	var tuples []Tuple
	for {
		tuple, err := iter.Next()
		if err == io.EOF {
			return tuples, nil
		}
		if err != nil {
			return nil, err
		}
		tuples = append(tuples, *tuple)
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
