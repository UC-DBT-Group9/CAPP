package kv

import (
	"testing"
	"time"
)

func TestSystemUsageSince(t *testing.T) {
	before := SystemUsage{
		Usage: Usage{StoredBytes: 100, Requests: 3, GetBytesReturned: 50,
			SelectBytesScanned: 200, SelectBytesReturned: 20, OutboundBytes: 5},
		CoordinatorUsage: CoordinatorUsage{ComputeTime: time.Minute},
	}
	after := SystemUsage{
		Usage: Usage{StoredBytes: 100, Requests: 5, GetBytesReturned: 150,
			SelectBytesScanned: 500, SelectBytesReturned: 50, OutboundBytes: 5},
		CoordinatorUsage: CoordinatorUsage{ComputeTime: 3 * time.Minute},
	}
	d := after.Since(before)

	if d.Requests != 2 || d.GetBytesReturned != 100 || d.SelectBytesScanned != 300 ||
		d.SelectBytesReturned != 30 || d.OutboundBytes != 0 {
		t.Errorf("counter deltas wrong: %+v", d.Usage)
	}
	if d.ComputeTime != 2*time.Minute {
		t.Errorf("ComputeTime = %v, want 2m", d.ComputeTime)
	}
	if d.StoredBytes != 100 {
		t.Errorf("StoredBytes = %d, want the current level 100", d.StoredBytes)
	}
}

func TestOraclePicksCheaper(t *testing.T) {
	rates := DefaultRates()
	gib := int64(1 << 30)

	// Selective filter: pushdown finishes much sooner, so it is cheaper even
	// after paying S3 Select charges.
	push := SystemUsage{
		Usage:            Usage{Requests: 1, SelectBytesScanned: 10 * gib, SelectBytesReturned: gib / 100},
		CoordinatorUsage: CoordinatorUsage{ComputeTime: 2 * time.Minute},
	}
	pull := SystemUsage{
		Usage:            Usage{Requests: 1, GetBytesReturned: 10 * gib},
		CoordinatorUsage: CoordinatorUsage{ComputeTime: 20 * time.Minute},
	}
	if got := Oracle(push.Cost(rates), pull.Cost(rates)).Strategy; got != StrategyPushdown {
		t.Errorf("selective filter: oracle picked %v, want pushdown", got)
	}

	// Filter that keeps almost everything: pushdown saves no time, so its
	// S3 Select charges make it the more expensive choice.
	push.SelectBytesReturned = 10 * gib
	push.ComputeTime = 20 * time.Minute
	if got := Oracle(push.Cost(rates), pull.Cost(rates)).Strategy; got != StrategyPull {
		t.Errorf("non-selective filter: oracle picked %v, want pull", got)
	}
}
