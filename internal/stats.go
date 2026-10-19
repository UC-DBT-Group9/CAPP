package kv

import (
	"fmt"
	"io"
)

// ColumnStats holds the summary statistics CAPP keeps for a single column.
// These are gathered once, when an object is loaded, and are what the cost
// model uses to estimate how many rows a predicate will keep before we decide
// whether to push it down to a storage worker or pull the raw data instead.
type ColumnStats struct {
	Name     string     // Column name, matching the schema attribute.
	DataType DataType   // Column type, copied from the schema.
	Count    int64      // Number of non-null values seen.
	NullN    int64      // Number of null values seen.
	Min      Value      // Smallest non-null value (zone-map low). Null if Count == 0.
	Max      Value      // Largest non-null value (zone-map high). Null if Count == 0.
	Distinct int64      // Estimated number of distinct non-null values.
	Hist     *Histogram // Equi-width histogram for numeric columns; nil otherwise.

	// seen backs the distinct estimate. We track exact distinct values up to
	// distinctCap; past that we stop growing the set and treat Distinct as a
	// lower bound, which keeps memory bounded on large columns.
	seen     map[string]struct{}
	overflow bool
}

// TableStats holds per-column statistics plus the table row count. This is the
// object the planner consults; one is built per stored CSV object.
type TableStats struct {
	Rows    int64
	Columns map[string]*ColumnStats
	order   []string // column order, for stable reporting
}

const (
	// distinctCap bounds how many distinct values we track exactly per column.
	distinctCap = 10000
	// histBuckets is the number of equi-width buckets per numeric column.
	histBuckets = 16
)

// CollectStats scans a CSV object once and returns its table statistics. The
// scan reuses the existing CSVObj.Iterator, so it honors the object's schema,
// header option, and null token.
//
// Numeric columns get an equi-width histogram, which gives sharper range
// estimates than min/max alone when the data is skewed (the refinement Mason
// raised in the planning meeting). Building it needs the min/max first, so we
// make two passes over the data: one to collect scalar stats, one to fill the
// histogram buckets.
func CollectStats(obj *CSVObj) (*TableStats, error) {
	if obj == nil {
		return nil, fmt.Errorf("CollectStats: nil object")
	}
	if len(obj.Schema.Attributes) == 0 {
		return nil, fmt.Errorf("CollectStats: object schema has no columns")
	}

	ts := &TableStats{Columns: make(map[string]*ColumnStats)}
	for _, attr := range obj.Schema.Attributes {
		cs := &ColumnStats{
			Name:     attr.Name,
			DataType: attr.DataType,
			Min:      NullValue(),
			Max:      NullValue(),
			seen:     make(map[string]struct{}),
		}
		ts.Columns[attr.Name] = cs
		ts.order = append(ts.order, attr.Name)
	}

	// Pass 1: row count, null count, min/max, distinct estimate.
	if err := scan(obj, func(i int, v Value) {
		if i == 0 {
			ts.Rows++
		}
		ts.Columns[obj.Schema.Attributes[i].Name].observe(v)
	}); err != nil {
		return nil, err
	}

	// Pass 2: fill histograms for numeric columns that have a usable range.
	for _, cs := range ts.Columns {
		if isNumeric(cs.Min) && cs.Count > 0 {
			cs.Hist = newHistogram(toFloat(cs.Min), toFloat(cs.Max), histBuckets)
		}
	}
	if err := scan(obj, func(i int, v Value) {
		cs := ts.Columns[obj.Schema.Attributes[i].Name]
		if cs.Hist != nil && isNumeric(v) {
			cs.Hist.add(toFloat(v))
		}
	}); err != nil {
		return nil, err
	}

	return ts, nil
}

// scan iterates the object once, calling fn for every (columnIndex, value).
func scan(obj *CSVObj, fn func(colIdx int, v Value)) error {
	iter := obj.Iterator()
	for {
		tuple, err := iter.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		for i := range obj.Schema.Attributes {
			fn(i, tuple.Values[i])
		}
	}
}

// observe folds a single value into the column's running scalar statistics.
func (cs *ColumnStats) observe(v Value) {
	if v.DataType == DataTypeNull {
		cs.NullN++
		return
	}
	cs.Count++

	if cs.Min.DataType == DataTypeNull {
		cs.Min = v
		cs.Max = v
	} else {
		if c, err := CompareValues(v, cs.Min); err == nil && c < 0 {
			cs.Min = v
		}
		if c, err := CompareValues(v, cs.Max); err == nil && c > 0 {
			cs.Max = v
		}
	}

	if !cs.overflow {
		key := v.statsKey()
		if _, ok := cs.seen[key]; !ok {
			if int64(len(cs.seen)) >= distinctCap {
				cs.overflow = true
			} else {
				cs.seen[key] = struct{}{}
			}
		}
	}
	cs.Distinct = int64(len(cs.seen))
}

// statsKey returns a string that uniquely identifies a scalar value for the
// purpose of distinct counting. Only the scalar CSV types are expected here.
func (v Value) statsKey() string {
	switch v.DataType {
	case DataTypeBool:
		return fmt.Sprintf("b:%t", v.Bool)
	case DataTypeInt:
		return fmt.Sprintf("i:%d", v.Int)
	case DataTypeFloat:
		return fmt.Sprintf("f:%g", v.Float)
	case DataTypeString:
		return "s:" + v.String
	default:
		return fmt.Sprintf("%d:%v", v.DataType, v)
	}
}

// Selectivity estimates the fraction of rows that satisfy a predicate, as a
// number in [0, 1]. It mirrors how KVStore.Select evaluates predicates, so the
// estimate lines up with what a pushed-down Select would actually return.
//
// Heuristics:
//   - col = literal     -> 1 / distinct(col)
//   - col > / < literal  -> histogram estimate if available, else linear
//     interpolation over [min, max]
//   - p AND q           -> sel(p) * sel(q)  (assumes independence)
//   - p OR q            -> sel(p) + sel(q) - sel(p)*sel(q)
//
// When a column has no usable statistics (e.g. all null, or a non-ordered
// type), it falls back to defaultSelectivity so the planner still gets a
// finite number.
func (ts *TableStats) Selectivity(pred *Expr) float64 {
	if pred == nil {
		return 1.0
	}
	return clamp01(ts.selectivity(pred))
}

const defaultSelectivity = 1.0 / 3.0 // used when stats can't inform a guess

func (ts *TableStats) selectivity(e *Expr) float64 {
	switch e.Op {
	case OpAnd:
		return ts.selectivity(e.Left) * ts.selectivity(e.Right)
	case OpOr:
		a := ts.selectivity(e.Left)
		b := ts.selectivity(e.Right)
		return a + b - a*b
	case OpEq:
		return ts.equalSelectivity(e)
	case OpGT, OpLT:
		return ts.rangeSelectivity(e)
	default:
		return defaultSelectivity
	}
}

// equalSelectivity handles "field = literal" (either operand order).
func (ts *TableStats) equalSelectivity(e *Expr) float64 {
	cs, _ := ts.fieldAndLiteral(e)
	if cs == nil || cs.Distinct <= 0 {
		return defaultSelectivity
	}
	return 1.0 / float64(cs.Distinct)
}

// rangeSelectivity handles "field > literal" and "field < literal". It uses the
// column histogram when present and falls back to uniform interpolation over
// [min, max]. Op is interpreted relative to the column: if the literal is on
// the left (literal > field), the comparison is flipped so the math still
// refers to the column.
func (ts *TableStats) rangeSelectivity(e *Expr) float64 {
	cs, lit := ts.fieldAndLiteral(e)
	if cs == nil || lit == nil || cs.Count == 0 {
		return defaultSelectivity
	}
	if !isNumeric(cs.Min) || !isNumeric(*lit) {
		return defaultSelectivity // only numeric ranges are interpolated
	}

	fieldOnLeft := e.Left != nil && e.Left.Op == OpField
	op := e.Op
	if !fieldOnLeft {
		// literal OP field  ==  field (flipped OP) literal
		if op == OpGT {
			op = OpLT
		} else {
			op = OpGT
		}
	}
	x := toFloat(*lit)

	if cs.Hist != nil {
		return cs.Hist.fractionGreater(x, op == OpGT)
	}

	// Fallback: uniform interpolation over [min, max].
	lo, hi := toFloat(cs.Min), toFloat(cs.Max)
	if hi <= lo {
		return defaultSelectivity
	}
	var frac float64
	if op == OpGT {
		frac = (hi - x) / (hi - lo)
	} else {
		frac = (x - lo) / (hi - lo)
	}
	return clamp01(frac)
}

// fieldAndLiteral pulls the column stats and literal value out of a binary
// comparison, in whichever operand order they appear. Returns nil stats if the
// expression is not a simple field-vs-literal comparison.
func (ts *TableStats) fieldAndLiteral(e *Expr) (*ColumnStats, *Value) {
	if e.Left == nil || e.Right == nil {
		return nil, nil
	}
	switch {
	case e.Left.Op == OpField && e.Right.Op == OpLiteral:
		return ts.column(e.Left.Path), &e.Right.Value
	case e.Left.Op == OpLiteral && e.Right.Op == OpField:
		return ts.column(e.Right.Path), &e.Left.Value
	default:
		return nil, nil
	}
}

// column resolves a field path to its stats. Only top-level columns are
// tracked, so a multi-segment path (into a nested object) has no stats.
func (ts *TableStats) column(path []string) *ColumnStats {
	if len(path) != 1 {
		return nil
	}
	return ts.Columns[path[0]]
}

// EstimateRows returns the expected number of rows a predicate keeps.
func (ts *TableStats) EstimateRows(pred *Expr) float64 {
	return float64(ts.Rows) * ts.Selectivity(pred)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
