package kv

import (
	"reflect"
	"testing"
)

func TestLogToPhys(t *testing.T) {
	want := &Limit{
		NumRows: 5,
		Input:   &LocalScan{Source: "users"}}
	logicalNode := LimitNode{
		NumRows: 5,
		Input:   TableScanNode{Source: "users"}}

	got, err := LogToPhys(logicalNode, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LogToPhys() = %v, want %v", got, want)
	}
}

func TestLogToPhys1(t *testing.T) {
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

	got, err := LogToPhys(logicalNode, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LogToPhys() = %v, want %v", got, want)
	}
}
