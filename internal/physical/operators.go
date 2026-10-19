package physical

import (
	"fmt"
	"slices"

	"github.com/UC-DBT-Group9/CAPP/internal"
)

type PhysicalPlanNode interface {
	physicalOp()
	NextBatch() (*kv.Schema, []kv.Tuple, error)
}

type Limit struct {
	NumRows uint64
	Input   PhysicalPlanNode
}

type LocalScan struct {
	Predicate *kv.Expr
	Project   []kv.NamedExpr
	Source    string
	coord     *kv.KVServer
}

type RemoteScan struct {
	Predicate *kv.Expr
	Project   []kv.NamedExpr
	Source    string
	coord     *kv.KVServer
}

func (Limit) physicalOp() {}
func NewLimitNode(rows uint64, input PhysicalPlanNode) Limit {
	return Limit{NumRows: rows, Input: input}
}
func (l Limit) NextBatch() (*kv.Schema, []kv.Tuple, error) {
	var collected uint64
	var tuples []kv.Tuple
	schema, batch, err := l.Input.NextBatch()
	if err != nil {
		return nil, nil, err
	}
	if uint64(len(batch)) < l.NumRows {
		return schema, batch, nil
	}
	collected += uint64(len(batch))
	tuples = slices.Concat(tuples, batch)

	for collected < l.NumRows {
		_, batch, err := l.Input.NextBatch()
		if err != nil {
			return nil, nil, err
		}
		if len(batch) == 0 {
			break
		}
		collected += uint64(len(batch))
		tuples = slices.Concat(tuples, batch)
	}

	return schema, tuples[:l.NumRows], nil
}

func (LocalScan) physicalOp() {}
func NewLocalScan(pred *kv.Expr, proj []kv.NamedExpr, source string, coord *kv.KVServer) LocalScan {
	return LocalScan{Predicate: pred, Project: proj, Source: source, coord: coord}
}
func (ls LocalScan) NextBatch() (*kv.Schema, []kv.Tuple, error) {
	var names []string
	var tuples []kv.Tuple
	for _, e := range ls.Project {
		names = append(names, e.Name)
	}
	reply, err := ls.coord.Select(ls.Source, nil, names)
	if err != nil {
		return nil, nil, err
	}
	for _, tuple := range reply.Tuples {
		if ls.Predicate != nil {
			res, err := ls.Predicate.Eval(tuple)
			if err != nil {
				return nil, nil, err
			}
			if res.DataType != kv.DataTypeBool {
				return nil, nil, fmt.Errorf("predicate must return bool, got %s", res.DataType)
			}
			if !res.Bool {
				continue
			}
		}
		tuples = append(tuples, tuple)
	}
	return &reply.Schema, tuples, nil
}

func (RemoteScan) physicalOp() {}
func NewRemoteScan(pred *kv.Expr, proj []kv.NamedExpr, source string, coord *kv.KVServer) RemoteScan {
	return RemoteScan{Predicate: pred, Project: proj, Source: source, coord: coord}
}
func (ls RemoteScan) NextBatch() (*kv.Schema, []kv.Tuple, error) {
	var names []string
	for _, e := range ls.Project {
		names = append(names, e.Name)
	}
	reply, err := ls.coord.Select(ls.Source, ls.Predicate, names)
	if err != nil {
		return nil, nil, err
	}
	return &reply.Schema, reply.Tuples, nil
}
