package kv

import (
	"math"
	"testing"
)

// peopleObj: ages 20,25,30,35,40 (min 20, max 40); one null city.
func peopleObj() *CSVObj {
	null := "NULL"
	return &CSVObj{
		Data: []byte("id,name,age,city\n" +
			"1,Ann,20,NYC\n" +
			"2,Bob,25,LA\n" +
			"3,Cy,30,NYC\n" +
			"4,Dee,35,SF\n" +
			"5,Ed,40,NULL\n"),
		Schema: Schema{Name: "people", Attributes: []Attribute{
			{Name: "id", DataType: DataTypeInt, Required: true},
			{Name: "name", DataType: DataTypeString, Required: true},
			{Name: "age", DataType: DataTypeInt, Required: true},
			{Name: "city", DataType: DataTypeString},
		}},
		Opts: CSVOpts{HasHeader: true, NullTok: &null},
	}
}

func TestCollectStats(t *testing.T) {
	ts, err := CollectStats(peopleObj())
	if err != nil {
		t.Fatalf("CollectStats() error = %v", err)
	}
	if ts.Rows != 5 {
		t.Errorf("Rows = %d, want 5", ts.Rows)
	}
	age := ts.Columns["age"]
	if age.Count != 5 || age.NullN != 0 {
		t.Errorf("age Count/NullN = %d/%d, want 5/0", age.Count, age.NullN)
	}
	if age.Min.Int != 20 || age.Max.Int != 40 {
		t.Errorf("age Min/Max = %d/%d, want 20/40", age.Min.Int, age.Max.Int)
	}
	if age.Distinct != 5 {
		t.Errorf("age Distinct = %d, want 5", age.Distinct)
	}
	if age.Hist == nil {
		t.Error("age should have a histogram")
	}
	city := ts.Columns["city"]
	if city.Count != 4 || city.NullN != 1 {
		t.Errorf("city Count/NullN = %d/%d, want 4/1", city.Count, city.NullN)
	}
	if city.Distinct != 3 {
		t.Errorf("city Distinct = %d, want 3", city.Distinct)
	}
	if city.Hist != nil {
		t.Error("string column city should not have a histogram")
	}
}

func TestSelectivityEqual(t *testing.T) {
	ts, _ := CollectStats(peopleObj())
	pred := Binary(OpEq, Field("city"), Literal(StringValue("NYC")))
	if got := ts.Selectivity(&pred); math.Abs(got-1.0/3.0) > 1e-9 {
		t.Errorf("Selectivity(city = NYC) = %v, want %v", got, 1.0/3.0)
	}
}

func TestSelectivityRange(t *testing.T) {
	ts, _ := CollectStats(peopleObj())
	// Out-of-range bounds are exact: nothing is > max, everything is > below-min.
	if got := selFor(ts, OpGT, 100); got != 0 {
		t.Errorf("Selectivity(age > 100) = %v, want 0", got)
	}
	if got := selFor(ts, OpGT, 0); got != 1 {
		t.Errorf("Selectivity(age > 0) = %v, want 1", got)
	}
	// A mid-range cut should land between 0 and 1, and GT+LT at the same point
	// should sum to roughly 1 (they partition the rows, boundary aside).
	gt := selFor(ts, OpGT, 30)
	lt := selFor(ts, OpLT, 30)
	if gt <= 0 || gt >= 1 {
		t.Errorf("Selectivity(age > 30) = %v, want strictly between 0 and 1", gt)
	}
	if math.Abs((gt+lt)-1) > 0.35 {
		t.Errorf("GT+LT at 30 = %v, want ~1", gt+lt)
	}
	// Monotonicity: a higher threshold keeps no more rows for '>'.
	if selFor(ts, OpGT, 35) > selFor(ts, OpGT, 25) {
		t.Error("Selectivity for '>' should be non-increasing in the threshold")
	}
}

func selFor(ts *TableStats, op Op, v int64) float64 {
	pred := Binary(op, Field("age"), Literal(IntValue(v)))
	return ts.Selectivity(&pred)
}

func TestSelectivityAndOr(t *testing.T) {
	ts, _ := CollectStats(peopleObj())
	gt30 := Binary(OpGT, Field("age"), Literal(IntValue(30)))
	eqNYC := Binary(OpEq, Field("city"), Literal(StringValue("NYC")))

	and := Binary(OpAnd, gt30, eqNYC)
	a, b := ts.Selectivity(&gt30), ts.Selectivity(&eqNYC)
	if got := ts.Selectivity(&and); math.Abs(got-a*b) > 1e-9 {
		t.Errorf("Selectivity(AND) = %v, want %v", got, a*b)
	}
	or := Binary(OpOr, gt30, eqNYC)
	if got := ts.Selectivity(&or); math.Abs(got-(a+b-a*b)) > 1e-9 {
		t.Errorf("Selectivity(OR) = %v, want %v", got, a+b-a*b)
	}
}

func TestEstimateRows(t *testing.T) {
	ts, _ := CollectStats(peopleObj())
	pred := Binary(OpGT, Field("age"), Literal(IntValue(30)))
	// 5 rows, a mid-range '>' keeps between 1 and 4 of them.
	if got := ts.EstimateRows(&pred); got <= 1 || got >= 4 {
		t.Errorf("EstimateRows(age > 30) = %v, want between 1 and 4", got)
	}
	if got := ts.EstimateRows(nil); got != 5 {
		t.Errorf("EstimateRows(nil) = %v, want 5", got)
	}
}
