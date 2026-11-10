package kv

import (
	"sync"
	"time"
)

const (
	bytesPerGiB          = 1 << 30
	defaultHoursPerMonth = 30 * 24

	defaultStorageGBMonthRate = 0.022
	defaultOutboundGBRate     = 0.09
	defaultComputeHourRate    = 2.128
	defaultSelectScanGBRate   = 0.002
	defaultSelectReturnGBRate = 0.0007
	defaultRequestsRate       = 0.0004 / 1000
)

type Usage struct {
	StoredBytes         int64
	StorageByteHours    float64
	Requests            uint64
	GetBytesReturned    int64
	SelectBytesScanned  int64
	SelectBytesReturned int64
	OutboundBytes       int64
}

type UsageTracker struct {
	mu      sync.Mutex
	usage   Usage
	updated time.Time
	now     func() time.Time
}

func NewUsageTracker() *UsageTracker { return &UsageTracker{now: time.Now} }

func (t *UsageTracker) accrue() {
	now := time.Now()
	if t.now != nil {
		now = t.now()
	}
	if !t.updated.IsZero() && now.After(t.updated) {
		t.usage.StorageByteHours += float64(t.usage.StoredBytes) * now.Sub(t.updated).Hours()
	}
	t.updated = now
}

func (t *UsageTracker) storageChanged(delta int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.accrue()
	t.usage.StoredBytes += delta
}
func (t *UsageTracker) request() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.Requests++
}

func (t *UsageTracker) recordGetBytesReturned(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.GetBytesReturned += bytes
}

func (t *UsageTracker) recordSelectBytesScanned(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.SelectBytesScanned += bytes
}

func (t *UsageTracker) recordSelectBytesReturned(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.SelectBytesReturned += bytes
}

func (t *UsageTracker) AddOutboundBytes(n int64) {
	if n <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.usage.OutboundBytes += n
}
func (t *UsageTracker) Snapshot() Usage {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.accrue()
	return t.usage
}

type Rates struct {
	StorageGBMonth, OutboundGB, ComputeHour           float64
	SelectScanGB, SelectReturnGB, RequestsPerThousand float64
	HoursPerMonth                                     float64
}

func DefaultRates() Rates {
	return Rates{
		StorageGBMonth:      defaultStorageGBMonthRate,
		OutboundGB:          defaultOutboundGBRate,
		ComputeHour:         defaultComputeHourRate,
		SelectScanGB:        defaultSelectScanGBRate,
		SelectReturnGB:      defaultSelectReturnGBRate,
		RequestsPerThousand: defaultRequestsRate,
		HoursPerMonth:       defaultHoursPerMonth,
	}
}

type UsageCost struct {
	Storage, Transfer, Compute, SelectScan, SelectReturn, Requests, Total float64
}

func (u Usage) Cost(r Rates) UsageCost {
	c := UsageCost{
		Storage:      u.StorageByteHours / bytesPerGiB / r.HoursPerMonth * r.StorageGBMonth,
		Transfer:     float64(u.OutboundBytes) / bytesPerGiB * r.OutboundGB,
		SelectScan:   float64(u.SelectBytesScanned) / bytesPerGiB * r.SelectScanGB,
		SelectReturn: float64(u.SelectBytesReturned) / bytesPerGiB * r.SelectReturnGB,
		Requests:     float64(u.Requests) * r.RequestsPerThousand,
	}
	c.Total = c.Storage + c.Transfer + c.Compute + c.SelectScan + c.SelectReturn + c.Requests
	return c
}

type CoordinatorUsage struct {
	ComputeTime time.Duration
}

func (u CoordinatorUsage) Cost(r Rates) UsageCost {
	compute := u.ComputeTime.Hours() * r.ComputeHour
	return UsageCost{Compute: compute, Total: compute}
}

type SystemUsage struct {
	Usage
	CoordinatorUsage
}

func (u SystemUsage) Cost(r Rates) UsageCost {
	cost := u.Usage.Cost(r)
	cost.Compute = u.CoordinatorUsage.Cost(r).Compute
	cost.Total += cost.Compute
	return cost
}
