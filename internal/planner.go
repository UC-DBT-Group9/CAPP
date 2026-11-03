package kv

import "fmt"

type Planner struct {
	coord *KVServer
}

func NewPlanner(coord *KVServer) *Planner {
	return &Planner{coord: coord}
}

func (p *Planner) Plan(logical LogicalPlanNode) (PhysicalPlanNode, error) {
	return LogToPhys(logical, p.coord)
}

func LogToPhys(logical LogicalPlanNode, coord *KVServer) (PhysicalPlanNode, error) {
	switch v := logical.(type) {
	case TableScanNode:
		if v.Source == "" {
			return nil, fmt.Errorf("table scan requires a source")
		}
		return NewLocalScan(v.Source, coord), nil
	case ProjectNode:
		input, err := LogToPhys(v.Input, coord)
		if err != nil {
			return nil, fmt.Errorf("project input: %w", err)
		}
		return &Project{Attributes: append([]string(nil), v.Attributes...), Input: input}, nil
	case FilterNode:
		input, err := LogToPhys(v.Input, coord)
		if err != nil {
			return nil, fmt.Errorf("filter input: %w", err)
		}
		return &Filter{Predicate: v.Predicate, Input: input}, nil
	case LimitNode:
		input, err := LogToPhys(v.Input, coord)
		if err != nil {
			return nil, fmt.Errorf("limit input: %w", err)
		}
		return NewLimitNode(v.NumRows, input), nil
	default:
		return nil, fmt.Errorf("unsupported logical node %T", logical)
	}
}
