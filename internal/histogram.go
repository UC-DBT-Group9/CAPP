package kv

// Histogram is a simple equi-width histogram over a numeric range [Lo, Hi].
// The range is split into len(Counts) buckets of equal width; Counts[i] is the
// number of observed values that fell in bucket i. It gives sharper range
// selectivity estimates than min/max alone when the data is skewed, because it
// knows where the values actually cluster, not just the extremes.
type Histogram struct {
	Lo     float64
	Hi     float64
	Counts []int64
	Total  int64
}

// newHistogram creates an empty histogram over [lo, hi] with n buckets.
// If the range is degenerate (hi <= lo) or n < 1, it falls back to a single
// bucket, which still behaves correctly (every value lands in it).
func newHistogram(lo, hi float64, n int) *Histogram {
	if n < 1 || hi <= lo {
		n = 1
	}
	return &Histogram{Lo: lo, Hi: hi, Counts: make([]int64, n)}
}

// bucketWidth returns the width of a single bucket.
func (h *Histogram) bucketWidth() float64 {
	if h.Hi <= h.Lo {
		return 0
	}
	return (h.Hi - h.Lo) / float64(len(h.Counts))
}

// add records one observed value, clamping out-of-range values into the end
// buckets (they can occur if stats were built from a different pass, though in
// practice the range comes from the same data).
func (h *Histogram) add(x float64) {
	idx := h.bucketOf(x)
	h.Counts[idx]++
	h.Total++
}

// bucketOf returns the bucket index a value falls into.
func (h *Histogram) bucketOf(x float64) int {
	w := h.bucketWidth()
	if w == 0 {
		return 0
	}
	idx := int((x - h.Lo) / w)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(h.Counts) {
		idx = len(h.Counts) - 1
	}
	return idx
}

// fractionGreater estimates the fraction of values satisfying (value > x) when
// greater is true, or (value < x) when greater is false. Within the bucket that
// contains x, it interpolates linearly, assuming values are spread uniformly
// across the bucket.
func (h *Histogram) fractionGreater(x float64, greater bool) float64 {
	if h.Total == 0 {
		return defaultSelectivity
	}
	if x <= h.Lo {
		if greater {
			return 1
		}
		return 0
	}
	if x >= h.Hi {
		if greater {
			return 0
		}
		return 1
	}

	w := h.bucketWidth()
	b := h.bucketOf(x)

	// Count values strictly above x: the fractional remainder of bucket b plus
	// every bucket after it.
	bucketLo := h.Lo + float64(b)*w
	posInBucket := (x - bucketLo) / w // 0..1 within bucket b
	var above float64
	above += float64(h.Counts[b]) * (1 - posInBucket)
	for i := b + 1; i < len(h.Counts); i++ {
		above += float64(h.Counts[i])
	}

	fracAbove := above / float64(h.Total)
	if greater {
		return clamp01(fracAbove)
	}
	return clamp01(1 - fracAbove)
}
