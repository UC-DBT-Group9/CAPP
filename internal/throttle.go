package kv

import "time"

// Throttle slows a storage worker down on purpose. When the coordinator and
// workers all run on one machine, data moves over localhost almost instantly,
// so pushdown and pull look the same in time. Throttling brings back the two
// costs that make the choice matter in the cloud:
//
//   - a limited network between storage and compute (BandwidthBytesPerSec),
//     which makes pulling raw data slow, and
//   - limited compute at storage (ScanBytesPerSec, SelectSlots), which makes
//     pushed-down work slow, especially when many Selects compete for it.
//
// The zero value means no throttling, so existing behavior is unchanged.
type Throttle struct {
	BandwidthBytesPerSec float64       // bytes/sec sent back to the coordinator; 0 = unlimited
	ScanBytesPerSec      float64       // bytes/sec a Select can scan at storage; 0 = unlimited
	SelectSlots          int           // max Selects running at once; 0 = unlimited
	RequestLatency       time.Duration // fixed delay added to every Get and Select
}

// transferDelay is how long sending n bytes takes at the throttled bandwidth.
func (t Throttle) transferDelay(n int64) time.Duration {
	return rateDelay(n, t.BandwidthBytesPerSec)
}

// scanDelay is how long a Select takes to scan n bytes at storage.
func (t Throttle) scanDelay(n int64) time.Duration {
	return rateDelay(n, t.ScanBytesPerSec)
}

func rateDelay(n int64, bytesPerSec float64) time.Duration {
	if bytesPerSec <= 0 || n <= 0 {
		return 0
	}
	return time.Duration(float64(n) / bytesPerSec * float64(time.Second))
}

// SetThrottle configures throttling for this store. Call it before the store
// starts serving requests.
func (store *KVStore) SetThrottle(t Throttle) {
	store.throttle = t
	store.slots = nil
	if t.SelectSlots > 0 {
		store.slots = make(chan struct{}, t.SelectSlots)
	}
}

// pause sleeps for d, using the store's injected sleep function when set so
// tests can check the delays without actually waiting.
func (store *KVStore) pause(d time.Duration) {
	if d <= 0 {
		return
	}
	if store.sleep != nil {
		store.sleep(d)
		return
	}
	time.Sleep(d)
}

// acquireSlot blocks until a Select slot is free, mimicking storage CPU that
// can only run a limited number of pushed-down operations at once. It returns
// the function that releases the slot.
func (store *KVStore) acquireSlot() func() {
	if store.slots == nil {
		return func() {}
	}
	store.slots <- struct{}{}
	return func() { <-store.slots }
}
