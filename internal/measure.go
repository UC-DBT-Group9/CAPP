package kv

import (
	"io"
	"strconv"
	"strings"
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

// EstimateTupleBytes returns the size of a tuple as CSV text. Pulling raw data
// ships the stored CSV object, and S3 Select bills by the CSV bytes it returns,
// so pushdown is measured in the same units. This keeps the planner comparing
// like with like when it weighs pushdown against pull.
func EstimateTupleBytes(t *Tuple) int64 {
	var n int64
	for i := range t.Values {
		n += estimateValueBytes(&t.Values[i])
	}
	return n
}

// estimateValueBytes is a value's length as a CSV field plus one separator
// (the comma or newline after it). Nulls count as empty fields.
func estimateValueBytes(v *Value) int64 {
	const sep = 1
	switch v.DataType {
	case DataTypeBool:
		return int64(len(strconv.FormatBool(v.Bool))) + sep
	case DataTypeInt:
		return int64(len(strconv.FormatInt(v.Int, 10))) + sep
	case DataTypeFloat:
		return int64(len(strconv.FormatFloat(v.Float, 'g', -1, 64))) + sep
	case DataTypeString:
		return csvFieldLen(v.String) + sep
	default: // DataTypeNull and anything unexpected
		return sep
	}
}

// csvFieldLen is the length of s once written as a CSV field: fields holding a
// comma, quote, or line break are wrapped in quotes, with inner quotes doubled.
func csvFieldLen(s string) int64 {
	if !strings.ContainsAny(s, ",\"\r\n") {
		return int64(len(s))
	}
	return int64(len(s) + strings.Count(s, `"`) + 2)
}

// MeasureSelect runs a Select over obj the given way and returns both the
// matching, projected tuples and a Measurement describing the cost. It is a
// self-contained reference implementation of the two strategies over a single
// in-memory object, so the planner and experiments can compare them without
// the RPC layer. The pushdown path returns only matching rows (as a worker
// would); the pull path reports the full object as bytes returned (as copying
// raw data would), then filters locally.
func MeasureSelect(obj *CSVObj, pred *Expr, project []string, strategy Strategy) ([]Tuple, *Measurement, error) {
	positions, _, err := ResolveProjection(obj.Schema, project)
	if err != nil {
		return nil, nil, err
	}

	pred, err = ResolvePredicate(obj.Schema, pred)
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

		match, err := MatchesPredicate(*tuple, pred)
		if err != nil {
			return nil, nil, err
		}

		projected := ProjectTuple(*tuple, positions)

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
