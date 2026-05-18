// Package traffic owns the periodic collection job, the
// cumulative→delta computation (with counter-reset detection), the
// aggregated usage queries, and the reset endpoints' service side.
package traffic

import (
	"sort"

	"github.com/cern/3xui-dashboard/internal/model"
)

// SumDeltas walks chronologically-ordered samples and returns the
// total upload + download bytes that accumulated across the series.
// Counter resets — a sample whose cumulative counter is lower than
// the prior sample — are treated as a fresh baseline: the new value
// is added as the delta and becomes the next "prior".
//
// This is the canonical way to turn the raw cum samples into a
// usage number that survives panel restarts and admin-initiated
// counter resets without producing negative deltas.
func SumDeltas(samples []model.TrafficSample) (up, down int64) {
	if len(samples) == 0 {
		return 0, 0
	}
	// Make sure we are in chronological order; the caller's queries
	// already sort but being defensive is cheap.
	if !sort.SliceIsSorted(samples, func(i, j int) bool {
		return samples[i].TakenAt.Before(samples[j].TakenAt)
	}) {
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].TakenAt.Before(samples[j].TakenAt)
		})
	}
	prevUp := samples[0].UpCumBytes
	prevDown := samples[0].DownCumBytes
	// The first sample's cumulative counter represents traffic that
	// accumulated before the dashboard started observing. We do
	// *not* count it — only the deltas we actually witnessed are
	// part of the returned total.
	for i := 1; i < len(samples); i++ {
		curUp := samples[i].UpCumBytes
		curDown := samples[i].DownCumBytes
		if curUp < prevUp {
			up += curUp // counter reset; full new value is delta
		} else {
			up += curUp - prevUp
		}
		if curDown < prevDown {
			down += curDown
		} else {
			down += curDown - prevDown
		}
		prevUp = curUp
		prevDown = curDown
	}
	return up, down
}

// BucketDeltas groups chronological samples into uniform time buckets
// of bucketSize duration. Each output bucket carries the up+down
// delta accumulated within it, with counter resets handled as in
// SumDeltas. Buckets with no sample pairs are omitted.
func BucketDeltas(samples []model.TrafficSample, bucketNanos int64) []BucketPoint {
	if bucketNanos <= 0 || len(samples) < 2 {
		return nil
	}
	if !sort.SliceIsSorted(samples, func(i, j int) bool {
		return samples[i].TakenAt.Before(samples[j].TakenAt)
	}) {
		sort.Slice(samples, func(i, j int) bool {
			return samples[i].TakenAt.Before(samples[j].TakenAt)
		})
	}
	type acc struct{ up, down int64 }
	buckets := map[int64]*acc{}

	prevUp := samples[0].UpCumBytes
	prevDown := samples[0].DownCumBytes
	for i := 1; i < len(samples); i++ {
		curUp := samples[i].UpCumBytes
		curDown := samples[i].DownCumBytes
		var dUp, dDown int64
		if curUp < prevUp {
			dUp = curUp
		} else {
			dUp = curUp - prevUp
		}
		if curDown < prevDown {
			dDown = curDown
		} else {
			dDown = curDown - prevDown
		}
		bk := samples[i].TakenAt.UnixNano() / bucketNanos
		a, ok := buckets[bk]
		if !ok {
			a = &acc{}
			buckets[bk] = a
		}
		a.up += dUp
		a.down += dDown
		prevUp = curUp
		prevDown = curDown
	}

	out := make([]BucketPoint, 0, len(buckets))
	for k, a := range buckets {
		out = append(out, BucketPoint{
			BucketStartUnix: k * bucketNanos / 1e9,
			Up:              a.up,
			Down:            a.down,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BucketStartUnix < out[j].BucketStartUnix })
	return out
}

// BucketPoint is one (time-bucketed) usage data point.
type BucketPoint struct {
	BucketStartUnix int64 `json:"bucket_start"` // unix seconds
	Up              int64 `json:"up"`
	Down            int64 `json:"down"`
}
