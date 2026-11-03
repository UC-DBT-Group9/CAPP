package kv

type LogicalPlanNode interface {
	logicalOp()
}

type LimitNode struct {
	NumRows uint64
	Input   LogicalPlanNode
}
type ProjectNode struct {
	Attributes []string
	Input      LogicalPlanNode
}
type FilterNode struct {
	Predicate Expr
	Input     LogicalPlanNode
}
type TableScanNode struct {
	Source string
}

func (LimitNode) logicalOp()     {}
func (ProjectNode) logicalOp()   {}
func (FilterNode) logicalOp()    {}
func (TableScanNode) logicalOp() {}
