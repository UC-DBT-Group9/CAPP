package kv

import "testing"

// numbersObj is a 100-row object with age = 0..99, used to check that pushdown
// returns fewer bytes than pull for a selective predicate.
func numbersObj() *CSVObj {
	var b []byte
	b = append(b, []byte("id,age\n")...)
	for i := 0; i < 100; i++ {
		row := make([]byte, 0, 12)
		row = appendInt(row, i)
		row = append(row, ',')
		row = appendInt(row, i) // age == id, 0..99
		row = append(row, '\n')
		b = append(b, row...)
	}
	return &CSVObj{
		Data: b,
		Schema: Schema{Name: "nums", Attributes: []Attribute{
			{Name: "id", DataType: DataTypeInt, Required: true},
			{Name: "age", DataType: DataTypeInt, Required: true},
		}},
		Opts: CSVOpts{HasHeader: true},
	}
}

func appendInt(b []byte, n int) []byte {
	if n == 0 {
		return append(b, '0')
	}
	var tmp [20]byte
	i := len(tmp)
	for n > 0 {
		i--
		tmp[i] = byte('0' + n%10)
		n /= 10
	}
	return append(b, tmp[i:]...)
}

func TestMeasureSelectPushVsPull(t *testing.T) {
	obj := numbersObj()
	pred := Binary(OpGT, Field("age"), Literal(IntValue(89))) // ages 90..99 -> 10 rows

	push, mp, err := MeasureSelect(obj, &pred, []string{"id"}, StrategyPushdown)
	if err != nil {
		t.Fatalf("pushdown: %v", err)
	}
	pull, mu, err := MeasureSelect(obj, &pred, []string{"id"}, StrategyPull)
	if err != nil {
		t.Fatalf("pull: %v", err)
	}

	// Both strategies return the same matching rows.
	if len(push) != 10 || len(pull) != 10 {
		t.Fatalf("rows: push=%d pull=%d, want 10/10", len(push), len(pull))
	}
	if mp.RowsReturned != 10 || mu.RowsReturned != 10 {
		t.Errorf("RowsReturned: push=%d pull=%d, want 10/10", mp.RowsReturned, mu.RowsReturned)
	}
	if mp.RowsScanned != 100 || mu.RowsScanned != 100 {
		t.Errorf("RowsScanned: push=%d pull=%d, want 100/100", mp.RowsScanned, mu.RowsScanned)
	}
	// The whole point: pushdown ships far fewer bytes than pulling raw data.
	if !(mp.BytesReturned < mu.BytesReturned) {
		t.Errorf("expected pushdown BytesReturned (%d) < pull (%d)", mp.BytesReturned, mu.BytesReturned)
	}
}

func TestMeasureSelectNoPredicate(t *testing.T) {
	obj := numbersObj()
	// With no predicate, pushdown and pull return every row; pushdown still
	// projects, so its bytes are no larger than pull's.
	push, mp, err := MeasureSelect(obj, nil, []string{"id"}, StrategyPushdown)
	if err != nil {
		t.Fatal(err)
	}
	if len(push) != 100 || mp.RowsReturned != 100 {
		t.Errorf("rows=%d RowsReturned=%d, want 100/100", len(push), mp.RowsReturned)
	}
}
