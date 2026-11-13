package kv

import "fmt"

type Planner struct {
	coord                 *KVServer
	policy                PlanPolicy
	Rates                 Rates
	ComputeBytesPerSecond float64
}

type PlanPolicy string

const (
	NeverPushdown  PlanPolicy = "never_pushdown"
	AlwaysPushdown PlanPolicy = "always_pushdown"
	CAPP           PlanPolicy = "capp"
)

func NewPlanner(coord *KVServer, policy PlanPolicy) *Planner {
	return &Planner{coord: coord, policy: policy, Rates: DefaultRates()}
}

func (p *Planner) Plan(logical LogicalPlanNode) (PhysicalPlanNode, error) {
	switch p.policy {
	case NeverPushdown:
		return lowerPhysical(logical, p.coord, false)
	case AlwaysPushdown:
		return lowerPhysical(logical, p.coord, true)
	case CAPP:
		return p.planCAPP(logical)
	default:
		return nil, fmt.Errorf("unknown planning policy %q", p.policy)
	}
}

func (p *Planner) planCAPP(logical LogicalPlanNode) (PhysicalPlanNode, error) {
	local, err := lowerPhysical(logical, p.coord, false)
	if err != nil {
		return nil, err
	}
	model, err := defaultCostModel(local, p.coord)
	if err != nil {
		return nil, err
	}
	model.Rates, model.ComputeBytesPerSecond = p.Rates, p.ComputeBytesPerSecond
	remote, err := lowerPhysical(logical, p.coord, true)
	if err != nil {
		return nil, err
	}
	a, err := model.Estimate(local)
	if err != nil {
		return nil, err
	}
	b, err := model.Estimate(remote)
	if err != nil {
		return nil, err
	}
	if b.Dollars < a.Dollars {
		return remote, nil
	}
	return local, nil
}

func lowerPhysical(logical LogicalPlanNode, coord *KVServer, pushdown bool) (PhysicalPlanNode, error) {
	switch v := logical.(type) {
	case TableScanNode:
		if pushdown {
			return NewRemoteScan(v.Source, coord), nil
		}
		return NewLocalScan(v.Source, coord), nil
	case ProjectNode:
		input, err := lowerPhysical(v.Input, coord, pushdown)
		if err != nil {
			return nil, fmt.Errorf("project input: %w", err)
		}
		if scan, ok := input.(*RemoteScan); ok && len(scan.Attributes) == 0 {
			scan.Attributes = append([]string{}, v.Attributes...)
			return scan, nil
		}
		return &Project{Attributes: append([]string{}, v.Attributes...), Input: input}, nil
	case FilterNode:
		input, err := lowerPhysical(v.Input, coord, pushdown)
		if err != nil {
			return nil, fmt.Errorf("filter input: %w", err)
		}
		if scan, ok := input.(*RemoteScan); ok && len(scan.Attributes) == 0 {
			pred := v.Predicate
			if scan.Predicate != nil {
				pred = Binary(OpAnd, *scan.Predicate, pred)
			}
			scan.Predicate = &pred
			return scan, nil
		}
		return &Filter{Predicate: v.Predicate, Input: input}, nil
	case LimitNode:
		input, err := lowerPhysical(v.Input, coord, pushdown)
		if err != nil {
			return nil, fmt.Errorf("limit input: %w", err)
		}
		return NewLimitNode(v.NumRows, input), nil
	default:
		return nil, fmt.Errorf("unsupported logical node %T", logical)
	}
}

func defaultCostModel(plan PhysicalPlanNode, coord *KVServer) (*CostModel, error) {
	if coord == nil {
		return nil, fmt.Errorf("default cost model requires a coordinator")
	}
	switch n := plan.(type) {
	case *LocalScan:
		stats, err := coord.GetStats(n.Source)
		if err != nil {
			return nil, fmt.Errorf("load statistics for %q: %w", n.Source, err)
		}
		return &CostModel{
			Tables: map[string]*TableStats{n.Source: stats},
			Rates:  DefaultRates(),
		}, nil
	case *Filter:
		return defaultCostModel(n.Input, coord)
	case *Project:
		return defaultCostModel(n.Input, coord)
	case *Limit:
		return defaultCostModel(n.Input, coord)
	default:
		return nil, fmt.Errorf("cannot load statistics for physical node %T", plan)
	}
}
