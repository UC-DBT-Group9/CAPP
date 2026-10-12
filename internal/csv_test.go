package kv

import (
	"reflect"
	"strings"
	"testing"
)

func TestCSVParse(t *testing.T) {
	null := "NULL"
	obj := CSVObj{
		Data: []byte("id,name,active,score\n1,\"Mason, Hall\",true,3.5\n2,Shiv,false,NULL\n"),
		Schema: Schema{Attributes: []Attribute{
			{Name: "id", DataType: DataTypeInt, Required: true},
			{Name: "name", DataType: DataTypeString, Required: true},
			{Name: "active", DataType: DataTypeBool, Required: true},
			{Name: "score", DataType: DataTypeFloat},
		}},
		Opts: CSVOpts{HasHeader: true, NullTok: &null},
	}

	got, err := obj.Parse()
	if err != nil {
		t.Fatal(err)
	}

	want := []Tuple{
		{Values: []Value{IntValue(1), StringValue("Mason, Hall"), BoolValue(true), FloatValue(3.5)}},
		{Values: []Value{IntValue(2), StringValue("Shiv"), BoolValue(false), NullValue()}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %#v, want %#v", got, want)
	}
}

func TestCSVParseErrors(t *testing.T) {
	null := "NULL"
	for _, tt := range []struct {
		name      string
		data      string
		hasHeader bool
		want      string
	}{
		{name: "header", data: "other\n1\n", hasHeader: true, want: "header column"},
		{name: "width", data: "1,2\n", want: "wrong number of fields"},
		{name: "type", data: "hello\n", want: "column id"},
		{name: "required", data: "NULL\n", want: "required attribute id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := CSVObj{
				Data: []byte(tt.data),
				Schema: Schema{Attributes: []Attribute{
					{Name: "id", DataType: DataTypeInt, Required: true},
				}},
				Opts: CSVOpts{HasHeader: tt.hasHeader, NullTok: &null},
			}
			if _, err := obj.Parse(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFromFile(t *testing.T) {
	null := "NULL"
	schema, err := NewSchema("test", []Attribute{
		{Name: "id", DataType: DataTypeInt, Required: true},
		{Name: "name", DataType: DataTypeString, Required: true},
		{Name: "age", DataType: DataTypeInt, Required: true},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}

	want := []Tuple{
		{[]Value{IntValue(1), StringValue("Mason"), IntValue(31)}},
		{[]Value{IntValue(2), StringValue("Shiv"), IntValue(25)}},
	}

	csvFromFile, err := CSVObjFromFile("testdata/test.csv", *schema, CSVOpts{HasHeader: true, NullTok: &null})
	if err != nil {
		t.Fatalf("CSVFromFile error: %v", err)
	}
	got, err := csvFromFile.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	for i := range want {
		for j := range want[i].Values {
			if !reflect.DeepEqual(want[i].Values[j], got[i].Values[j]) {
				t.Fatalf("Parse() = %#v, want %#v", got[i].Values[j], want[i].Values[j])
			}
		}
	}
}
