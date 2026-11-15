package kv

import "fmt"

type PhysicalPlanNode interface {
	physicalOp()
	Execute() (*Schema, []Tuple, error)
}

type Limit struct {
	NumRows uint64
	Input   PhysicalPlanNode
}

type Project struct {
	Attributes []string
	Input      PhysicalPlanNode
}

type Filter struct {
	Predicate Expr
	Input     PhysicalPlanNode
}

type LocalScan struct {
	Source string
	coord  *KVServer
}

type RemoteScan struct {
	Predicate  *Expr
	Attributes []string
	Source     string
	coord      *KVServer
}

func (*Limit) physicalOp() {}
func NewLimitNode(rows uint64, input PhysicalPlanNode) *Limit {
	return &Limit{NumRows: rows, Input: input}
}
func (l *Limit) Execute() (*Schema, []Tuple, error) {
	schema, tuples, err := l.Input.Execute()
	if err != nil {
		return nil, nil, err
	}
	if uint64(len(tuples)) > l.NumRows {
		tuples = tuples[:l.NumRows]
	}
	return schema, tuples, nil
}

func (*Project) physicalOp() {}
func (p *Project) Execute() (*Schema, []Tuple, error) {
	schema, input, err := p.Input.Execute()
	if err != nil {
		return nil, nil, err
	}
	positions, output, err := ResolveProjection(*schema, p.Attributes)
	if err != nil {
		return nil, nil, err
	}
	var tuples []Tuple
	for _, tuple := range input {
		tuples = append(tuples, ProjectTuple(tuple, positions))
	}
	return &output, tuples, nil
}

func (*Filter) physicalOp() {}
func (f *Filter) Execute() (*Schema, []Tuple, error) {
	schema, input, err := f.Input.Execute()
	if err != nil {
		return nil, nil, err
	}
	predicate, err := ResolvePredicate(*schema, &f.Predicate)
	if err != nil {
		return nil, nil, err
	}
	var tuples []Tuple
	for _, tuple := range input {
		match, err := MatchesPredicate(tuple, predicate)
		if err != nil {
			return nil, nil, err
		}
		if match {
			tuples = append(tuples, tuple)
		}
	}
	return schema, tuples, nil
}

func (*LocalScan) physicalOp() {}
func NewLocalScan(source string, coord *KVServer) *LocalScan {
	return &LocalScan{Source: source, coord: coord}
}
func (ls *LocalScan) Execute() (*Schema, []Tuple, error) {
	if ls.coord == nil {
		return nil, nil, fmt.Errorf("local scan requires a coordinator")
	}
	csv, err := ls.coord.Get(ls.Source)
	if err != nil {
		return nil, nil, err
	}
	tuples, err := csv.Parse()
	if err != nil {
		return nil, nil, err
	}
	return &csv.Schema, tuples, nil
}

func (*RemoteScan) physicalOp() {}
func NewRemoteScan(source string, coord *KVServer) *RemoteScan {
	return &RemoteScan{Source: source, coord: coord}
}
func (rs *RemoteScan) Execute() (*Schema, []Tuple, error) {
	if rs.coord == nil {
		return nil, nil, fmt.Errorf("remote scan requires a coordinator")
	}
	reply, err := rs.coord.Select(rs.Source, rs.Predicate, rs.Attributes)
	if err != nil {
		return nil, nil, err
	}
	return &reply.Schema, reply.Tuples, nil
}
