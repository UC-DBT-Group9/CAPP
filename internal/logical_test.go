package kv

import (
	"reflect"
	"testing"
)

func TestPlannerLowersLimit(t *testing.T) {
	want := &Limit{
		NumRows: 5,
		Input:   &LocalScan{Source: "users"}}
	logicalNode := LimitNode{
		NumRows: 5,
		Input:   TableScanNode{Source: "users"}}

	got, err := NewPlanner(nil, NeverPushdown).Plan(logicalNode)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %v, want %v", got, want)
	}
}

func TestPlannerLowersProjectionWithLimit(t *testing.T) {
	want := &Limit{
		NumRows: 5,
		Input: &Project{
			Attributes: []string{"id"},
			Input:      &LocalScan{Source: "users"},
		}}

	logicalNode := LimitNode{
		NumRows: 5,
		Input: ProjectNode{
			Attributes: []string{"id"},
			Input:      TableScanNode{Source: "users"}}}

	got, err := NewPlanner(nil, NeverPushdown).Plan(logicalNode)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %v, want %v", got, want)
	}
}
