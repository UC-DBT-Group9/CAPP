package kv

import (
	"reflect"
	"testing"
)

type testSource struct {
	schema Schema
	rows   []Tuple
	err    error
}

func (*testSource) physicalOp() {}
func (s *testSource) Execute() (*Schema, []Tuple, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return &s.schema, s.rows, nil
}

func idRows(ids ...int64) []Tuple {
	var rows []Tuple
	for _, id := range ids {
		rows = append(rows, Tuple{Values: []Value{IntValue(id)}})
	}
	return rows
}

func executePlan(t *testing.T, node PhysicalPlanNode) (*Schema, []Tuple) {
	t.Helper()
	schema, rows, err := node.Execute()
	if err != nil {
		t.Fatal(err)
	}
	return schema, rows
}

func TestFilterThenLimit(t *testing.T) {
	source := &testSource{
		schema: Schema{Attributes: []Attribute{{Name: "id", DataType: DataTypeInt}}},
		rows:   idRows(1, 2, 3, 4, 5),
	}
	filter := &Filter{
		Predicate: Binary(OpGT, Field("id"), Literal(IntValue(2))),
		Input:     source,
	}
	_, got := executePlan(t, NewLimitNode(2, filter))
	if !reflect.DeepEqual(got, idRows(3, 4)) {
		t.Fatalf("rows = %v; want [3,4]", got)
	}
}
