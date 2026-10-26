package execution

import "github.com/UC-DBT-Group9/CAPP/internal/physical"

type RemoteScan struct {
	plan physical.RemoteScan
}

func NewRemoteScan(plan physical.RemoteScan) *RemoteScan {
	return &RemoteScan{plan: plan}
}

// Pull batch from remote storage.
func (rs *RemoteScan) NextBatch() (TupleBatch, error) {
}
