package kv

import (
	"math"
	"testing"
)

// For SELECT * with no filter, pushdown and pull move the same rows, so the
// cost model's transfer estimates for the two should match. Pull is measured
// from the raw CSV object, so pushdown must be estimated in CSV bytes too, or
// CAPP compares two different units and is biased toward one side.
func TestCostModelPushAndPullSameUnits(t *testing.T) {
	obj := numbersObj()
	stats, err := CollectStats(obj)
	if err != nil {
		t.Fatal(err)
	}
	model := &CostModel{Tables: map[string]*TableStats{"nums": stats}, Rates: DefaultRates()}

	pull, err := model.Estimate(NewLocalScan("nums", nil))
	if err != nil {
		t.Fatal(err)
	}
	push, err := model.Estimate(NewRemoteScan("nums", nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SELECT * transfer estimate: pushdown=%.0f bytes, pull=%.0f bytes", push.BytesTransferred, pull.BytesTransferred)

	// Pull includes the CSV header line; pushdown returns only rows. Allow 2%.
	if diff := math.Abs(push.BytesTransferred-pull.BytesTransferred) / pull.BytesTransferred; diff > 0.02 {
		t.Errorf("pushdown %.0f vs pull %.0f bytes differ by %.0f%%; they should be in the same units",
			push.BytesTransferred, pull.BytesTransferred, diff*100)
	}
}

func TestEstimateValueBytesIsCSVSize(t *testing.T) {
	tests := []struct {
		name string
		v    Value
		want int64
	}{
		{"int one digit", IntValue(7), 2},            // "7,"
		{"int two digits", IntValue(90), 3},          // "90,"
		{"negative int", IntValue(-5), 3},            // "-5,"
		{"float", FloatValue(0.04), 5},               // "0.04,"
		{"bool", BoolValue(true), 5},                 // "true,"
		{"string", StringValue("abc"), 4},            // "abc,"
		{"string with comma", StringValue("a,b"), 6}, // "\"a,b\","
		{"string with quote", StringValue(`a"b`), 7}, // "\"a\"\"b\","
		{"null", NullValue(), 1},                     // ","
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estimateValueBytes(&tt.v); got != tt.want {
				t.Errorf("estimateValueBytes(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

// The query on our slides: SELECT id FROM nums WHERE age > 89 keeps ids 90..99.
func TestPushVsPullBytesForSlides(t *testing.T) {
	obj := numbersObj()
	pred := Binary(OpGT, Field("age"), Literal(IntValue(89)))
	_, mp, err := MeasureSelect(obj, &pred, []string{"id"}, StrategyPushdown)
	if err != nil {
		t.Fatal(err)
	}
	_, mu, err := MeasureSelect(obj, &pred, []string{"id"}, StrategyPull)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pushdown: rows=%d bytes=%d | pull: rows=%d bytes=%d", mp.RowsReturned, mp.BytesReturned, mu.RowsReturned, mu.BytesReturned)
	// Ten ids of two digits plus a newline each.
	if mp.BytesReturned != 30 {
		t.Errorf("pushdown bytes = %d, want 30", mp.BytesReturned)
	}
	// Every data row of the CSV object (the object minus its header line).
	if want := int64(len(obj.Data) - len("id,age\n")); mu.BytesReturned != want {
		t.Errorf("pull bytes = %d, want %d", mu.BytesReturned, want)
	}
}
