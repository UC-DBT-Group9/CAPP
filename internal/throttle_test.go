package kv

import (
	"sync"
	"testing"
	"time"
)

// throttledStore returns a store holding numbersObj under "nums", with the
// given throttle and a fake sleep that records delays instead of waiting.
func throttledStore(t *testing.T, th Throttle) (*KVStore, *[]time.Duration) {
	t.Helper()
	store := NewKVStore()
	store.SetThrottle(th)
	var mu sync.Mutex
	var slept []time.Duration
	store.sleep = func(d time.Duration) {
		mu.Lock()
		slept = append(slept, d)
		mu.Unlock()
	}
	var ok PutReply
	if err := store.Put(&PutArgs{Key: "nums", CSVObj: *numbersObj()}, &ok); err != nil || !ok {
		t.Fatalf("Put: ok=%v err=%v", ok, err)
	}
	return store, &slept
}

func total(ds []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum
}

func TestNoThrottleNoDelay(t *testing.T) {
	store, slept := throttledStore(t, Throttle{})
	var get GetReply
	if err := store.Get(&GetArgs{Key: "nums"}, &get); err != nil {
		t.Fatal(err)
	}
	var sel SelectReply
	if err := store.Select(&SelectArgs{Key: "nums"}, &sel); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 0 {
		t.Errorf("unthrottled store slept %v", *slept)
	}
}

func TestGetPaysBandwidthForWholeObject(t *testing.T) {
	const bw = 1000.0 // bytes per second
	store, slept := throttledStore(t, Throttle{BandwidthBytesPerSec: bw, RequestLatency: 5 * time.Millisecond})
	size := int64(len(numbersObj().Data))

	var get GetReply
	if err := store.Get(&GetArgs{Key: "nums"}, &get); err != nil {
		t.Fatal(err)
	}
	want := 5*time.Millisecond + rateDelay(size, bw)
	if got := total(*slept); got != want {
		t.Errorf("Get delay = %v, want %v", got, want)
	}
}

func TestSelectPaysScanAndOnlyReturnedBytes(t *testing.T) {
	const bw, scan = 1000.0, 4000.0
	store, slept := throttledStore(t, Throttle{BandwidthBytesPerSec: bw, ScanBytesPerSec: scan})
	size := int64(len(numbersObj().Data))

	pred := Binary(OpGT, Field("age"), Literal(IntValue(89))) // 10 of 100 rows
	var sel SelectReply
	if err := store.Select(&SelectArgs{Key: "nums", Predicate: &pred, Project: []string{"id"}}, &sel); err != nil {
		t.Fatal(err)
	}
	var returned int64
	for i := range sel.Tuples {
		returned += EstimateTupleBytes(&sel.Tuples[i])
	}
	want := rateDelay(size, scan) + rateDelay(returned, bw)
	if got := total(*slept); got != want {
		t.Errorf("Select delay = %v, want %v", got, want)
	}

	// The point of the throttle: for a selective filter, pushing down costs
	// less network time than pulling the whole object would.
	if rateDelay(returned, bw) >= rateDelay(size, bw) {
		t.Errorf("pushdown transfer %v should be less than full pull %v",
			rateDelay(returned, bw), rateDelay(size, bw))
	}
}

func TestSelectSlotsLimitConcurrency(t *testing.T) {
	store := NewKVStore()
	store.SetThrottle(Throttle{SelectSlots: 1})

	var mu sync.Mutex
	running, peak := 0, 0
	store.sleep = func(time.Duration) {}
	var ok PutReply
	if err := store.Put(&PutArgs{Key: "nums", CSVObj: *numbersObj()}, &ok); err != nil {
		t.Fatal(err)
	}

	// Wrap the slot logic directly: with one slot, two holders never overlap.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := store.acquireSlot()
			mu.Lock()
			running++
			if running > peak {
				peak = running
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
			release()
		}()
	}
	wg.Wait()
	if peak != 1 {
		t.Errorf("peak concurrent Selects = %d, want 1", peak)
	}
}
