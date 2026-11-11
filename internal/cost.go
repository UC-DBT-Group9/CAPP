package kv

import (
	"fmt"
	"math"
)

const DefaultComputeBytesPerSecond = 100 * 1024 * 1024

type CostEstimate struct {
	Rows             float64
	RowBytes         float64
	OutputBytes      float64
	BytesScanned     float64
	BytesTransferred float64
	ComputeHours     float64
	Dollars          float64
}

type CostModel struct {
	Tables                map[string]*TableStats
	Rates                 Rates
	ComputeBytesPerSecond float64
}

func (m *CostModel) Estimate(plan PhysicalPlanNode) (CostEstimate, error) {
	if m.ComputeBytesPerSecond == 0 {
		m.ComputeBytesPerSecond = DefaultComputeBytesPerSecond
	}
	s, err := m.estimate(plan)
	if err != nil {
		return CostEstimate{}, err
	}
	s.cost.OutputBytes = s.cost.Rows * s.cost.RowBytes
	s.cost.ComputeHours = (s.cost.BytesScanned + s.cost.BytesTransferred) / m.ComputeBytesPerSecond / 3600
	s.cost.Dollars += s.cost.ComputeHours * m.Rates.ComputeHour
	return s.cost, nil
}

type costState struct {
	cost     CostEstimate
	schema   Schema
	columns  []*ColumnStats
	baseRows int64
}

func (m *CostModel) estimate(plan PhysicalPlanNode) (costState, error) {
	switch n := plan.(type) {
	case *LocalScan:
		return m.scan(n.Source, nil)
	case *RemoteScan:
		return m.scan(n.Source, n)
	case *Filter:
		s, err := m.estimate(n.Input)
		if err != nil {
			return s, err
		}
		pred, err := ResolvePredicate(s.schema, &n.Predicate)
		if err != nil {
			return costState{}, err
		}
		stats := &TableStats{Rows: s.baseRows, Columns: make(map[string]*ColumnStats)}
		for i, cs := range s.columns {
			name := fmt.Sprint(i)
			stats.order = append(stats.order, name)
			stats.Columns[name] = cs
		}
		s.cost.Rows *= stats.Selectivity(pred)
		return s, nil
	case *Project:
		s, err := m.estimate(n.Input)
		if err != nil {
			return s, err
		}
		positions, schema, err := ResolveProjection(s.schema, n.Attributes)
		if err != nil {
			return costState{}, err
		}
		columns := make([]*ColumnStats, len(positions))
		s.cost.RowBytes = 0
		for i, pos := range positions {
			columns[i] = s.columns[pos]
			s.cost.RowBytes += averageColumnBytes(columns[i], s.baseRows)
		}
		s.schema, s.columns = schema, columns
		return s, nil
	case *Limit:
		s, err := m.estimate(n.Input)
		if err != nil {
			return s, err
		}
		s.cost.Rows = math.Min(s.cost.Rows, float64(n.NumRows))
		return s, nil
	}
	return costState{}, fmt.Errorf("cannot estimate physical node %T", plan)
}

func (m *CostModel) scan(source string, remote *RemoteScan) (costState, error) {
	t := m.Tables[source]
	if t == nil {
		return costState{}, fmt.Errorf("no table statistics for %q", source)
	}
	s := costState{schema: t.Schema, baseRows: t.Rows}
	for _, attr := range t.Schema.Attributes {
		cs := t.Columns[attr.Name]
		s.columns = append(s.columns, cs)
		s.cost.RowBytes += averageColumnBytes(cs, t.Rows)
	}
	s.cost.Rows = float64(t.Rows)
	s.cost.BytesScanned = float64(t.ObjectBytes)
	s.cost.BytesTransferred = float64(t.ObjectBytes)
	s.cost.Dollars = m.Rates.RequestsPerThousand
	if remote != nil {
		pred, err := ResolvePredicate(t.Schema, remote.Predicate)
		if err != nil {
			return costState{}, err
		}
		positions, schema, err := ResolveProjection(t.Schema, remote.Attributes)
		if err != nil {
			return costState{}, err
		}
		if pred != nil {
			s.cost.Rows *= t.Selectivity(pred)
		}
		s.cost.RowBytes = 0
		columns := make([]*ColumnStats, len(positions))
		for i, pos := range positions {
			columns[i] = s.columns[pos]
			s.cost.RowBytes += averageColumnBytes(s.columns[pos], t.Rows)
		}
		s.schema, s.columns = schema, columns
		s.cost.BytesTransferred = s.cost.Rows * s.cost.RowBytes
		s.cost.Dollars += s.cost.BytesScanned/bytesPerGiB*m.Rates.SelectScanGB +
			s.cost.BytesTransferred/bytesPerGiB*m.Rates.SelectReturnGB
	}
	s.cost.Dollars += s.cost.BytesTransferred / bytesPerGiB * m.Rates.OutboundGB
	return s, nil
}

func averageColumnBytes(cs *ColumnStats, rows int64) float64 {
	if rows == 0 {
		return 0
	}
	return float64(cs.TupleBytes) / float64(rows)
}
