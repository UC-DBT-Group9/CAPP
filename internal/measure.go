package kv

import (
	"fmt"
	"io"
	"time"
)

// Strategy names the two ways CAPP can answer a Select, which the planner will
// eventually choose between.
type Strategy string

const (
	// StrategyPushdown runs the predicate and projection on the storage worker
	// and ships back only matching, projected rows (what S3 Select would do).
	StrategyPushdown Strategy = "pushdown"
	// StrategyPull copies the whole object to the coordinator and filters there.
	StrategyPull Strategy = "pull"
)

// Measurement records what one Select actually cost, regardless of strategy.
// These are the raw numbers the cost model turns into a time/dollar estimate,
// and the "partial results" we can show for Report 2: run the same query each
// way and compare BytesReturned and Elapsed.
type Measurement struct {
	Strategy      Strategy
	Key           string
	RowsScanned   int64         // rows read from storage
	RowsReturned  int64         // rows that matched the predicate
	BytesScanned  int64         // bytes read from storage (≈ whole object)
	BytesReturned int64         // bytes crossing the network back to compute
	Elapsed       time.Duration // wall-clock time for the operation
}

// EstimateTupleBytes returns an approximate serialized size for a tuple,
// counting a fixed overhead per value plus the length of variable-width data.
// It is deliberately simple: the goal is a consistent proxy for "how much data
// crosses the network", not an exact wire size.
func EstimateTupleBytes(t *Tuple) int64 {
	var n int64
	for i := range t.Values {
		n += estimateValueBytes(&t.Values[i])
	}
	return n
}

const valueOverhead = 2 // bytes of per-value framing in our rough model

func estimateValueBytes(v *Value) int64 {
	switch v.DataType {
	case DataTypeNull:
		return valueOverhead
	case DataTypeBool:
		return valueOverhead + 1
	case DataTypeInt:
		return valueOverhead + 8
	case DataTypeFloat:
		return valueOverhead + 8
	case DataTypeString:
		return valueOverhead + int64(len(v.String))
	case DataTypeBytes:
		return valueOverhead + int64(len(v.Bytes))
	default:
		return valueOverhead
	}
}

// MeasureSelect runs a Select over obj the given way and returns both the
// matching, projected tuples and a Measurement describing the cost. It is a
// self-contained reference implementation of the two strategies over a single
// in-memory object, so the planner and experiments can compare them without
// the RPC layer. The pushdown path returns only matching rows (as a worker
// would); the pull path reports the full object as bytes returned (as copying
// raw data would), then filters locally.
func MeasureSelect(obj *CSVObj, pred *Expr, project []string, strategy Strategy) ([]Tuple, *Measurement, error) {
	positions, _, err := resolveProjection(obj, project)
	if err != nil {
		return nil, nil, err
	}

	m := &Measurement{Strategy: strategy, BytesScanned: int64(len(obj.Data))}
	start := time.Now()

	var out []Tuple
	iter := obj.Iterator()
	for {
		tuple, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		m.RowsScanned++

		match, err := matches(obj, tuple, pred)
		if err != nil {
			return nil, nil, err
		}

		projected := projectTuple(tuple, positions)

		switch strategy {
		case StrategyPushdown:
			// Filtering happens at storage; only matching rows are returned.
			if match {
				m.RowsReturned++
				m.BytesReturned += EstimateTupleBytes(&projected)
				out = append(out, projected)
			}
		case StrategyPull:
			// Raw data is shipped first (every scanned row counts against the
			// network), then filtered at the compute side.
			m.BytesReturned += EstimateTupleBytes(tuple)
			if match {
				m.RowsReturned++
				out = append(out, projected)
			}
		}
	}

	m.Elapsed = time.Since(start)
	return out, m, nil
}

// --- small helpers, kept local so this file is self-contained ---

func resolveProjection(obj *CSVObj, project []string) ([]int, Schema, error) {
	out := Schema{Name: obj.Schema.Name}
	var positions []int
	if len(project) == 0 {
		for i, attr := range obj.Schema.Attributes {
			positions = append(positions, i)
			out.Attributes = append(out.Attributes, attr)
		}
		return positions, out, nil
	}
	for _, name := range project {
		pos := -1
		for i, attr := range obj.Schema.Attributes {
			if attr.Name == name {
				pos = i
				break
			}
		}
		if pos == -1 {
			return nil, out, fmt.Errorf("unknown projection column %q", name)
		}
		positions = append(positions, pos)
		out.Attributes = append(out.Attributes, obj.Schema.Attributes[pos])
	}
	return positions, out, nil
}

func projectTuple(t *Tuple, positions []int) Tuple {
	p := Tuple{Values: make([]Value, len(positions))}
	for i, pos := range positions {
		p.Values[i] = t.Values[pos]
	}
	return p
}

func matches(obj *CSVObj, t *Tuple, pred *Expr) (bool, error) {
	if pred == nil {
		return true, nil
	}
	fields := make(map[string]Value, len(t.Values))
	for i, attr := range obj.Schema.Attributes {
		fields[attr.Name] = t.Values[i]
	}
	res, err := pred.Eval(ObjectValue(fields))
	if err != nil {
		return false, err
	}
	if res.DataType != DataTypeBool {
		return false, fmt.Errorf("predicate must return bool, got %s", res.DataType)
	}
	return res.Bool, nil
}
