package execution

import kv "github.com/UC-DBT-Group9/CAPP/internal"

type TupleBatch struct {
	Tuples []kv.Tuple
}

type PhysicalOperator interface {
	NextBatch() (TupleBatch, error)
}
