package node

import (
	"sort"
	"sync"
	"time"
)

// MetricSample is one (timestamp, cpu, mem) tuple stored in the ring.
type MetricSample struct {
	Time time.Time `json:"time"`
	CPU  float64   `json:"cpu"`
	Mem  float64   `json:"mem"`
}

// MetricsStore is the in-memory per-node ring buffer. With a 30s probe
// cadence the default cap of 720 samples covers ~6 hours of history
// which is enough for charting; longer windows are persisted as
// traffic_samples by group 8.
type MetricsStore struct {
	mu     sync.RWMutex
	cap    int
	tables map[int64]*ring
}

// NewMetricsStore returns an empty store. cap is the per-node sample
// ceiling; pass 0 for the default of 720.
func NewMetricsStore(cap int) *MetricsStore {
	if cap <= 0 {
		cap = 720
	}
	return &MetricsStore{cap: cap, tables: make(map[int64]*ring)}
}

// Append records one sample for nodeID. Drops the oldest entry once
// the ring is full.
func (m *MetricsStore) Append(nodeID int64, ts time.Time, cpu, mem float64) {
	m.mu.Lock()
	r, ok := m.tables[nodeID]
	if !ok {
		r = newRing(m.cap)
		m.tables[nodeID] = r
	}
	r.push(MetricSample{Time: ts, CPU: cpu, Mem: mem})
	m.mu.Unlock()
}

// Drop forgets every sample for a node — called on Service.Delete.
func (m *MetricsStore) Drop(nodeID int64) {
	m.mu.Lock()
	delete(m.tables, nodeID)
	m.mu.Unlock()
}

// Raw returns every sample for the node within [from, to], oldest
// first. Returns nil if the node has never been sampled.
func (m *MetricsStore) Raw(nodeID int64, from, to time.Time) []MetricSample {
	m.mu.RLock()
	r, ok := m.tables[nodeID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	return r.window(from, to)
}

// Bucketed returns the average CPU/Mem grouped into uniform time
// buckets of bucket size. Each output point is anchored at the bucket
// start. Empty buckets are skipped — callers wanting a dense series
// should fill gaps on their side.
//
// Returns nil + nil when the node has no samples or bucket <= 0.
func (m *MetricsStore) Bucketed(nodeID int64, from, to time.Time, bucket time.Duration) []MetricSample {
	if bucket <= 0 {
		return nil
	}
	raw := m.Raw(nodeID, from, to)
	if len(raw) == 0 {
		return nil
	}
	type acc struct {
		cpu float64
		mem float64
		n   int
	}
	buckets := make(map[int64]*acc)
	for _, s := range raw {
		k := s.Time.UnixNano() / int64(bucket)
		a, ok := buckets[k]
		if !ok {
			a = &acc{}
			buckets[k] = a
		}
		a.cpu += s.CPU
		a.mem += s.Mem
		a.n++
	}

	out := make([]MetricSample, 0, len(buckets))
	for k, a := range buckets {
		out = append(out, MetricSample{
			Time: time.Unix(0, k*int64(bucket)).UTC(),
			CPU:  a.cpu / float64(a.n),
			Mem:  a.mem / float64(a.n),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// ---- ring buffer (internal) ------------------------------------------------

type ring struct {
	cap  int
	data []MetricSample
	head int // next write position
	full bool
}

func newRing(cap int) *ring {
	return &ring{cap: cap, data: make([]MetricSample, cap)}
}

func (r *ring) push(s MetricSample) {
	r.data[r.head] = s
	r.head = (r.head + 1) % r.cap
	if r.head == 0 {
		r.full = true
	}
}

// window returns samples within [from, to] in chronological order.
// from/to are inclusive; pass zero time on either side to skip that
// bound.
func (r *ring) window(from, to time.Time) []MetricSample {
	n := r.head
	if r.full {
		n = r.cap
	}
	out := make([]MetricSample, 0, n)
	read := func(idx int) {
		s := r.data[idx]
		if s.Time.IsZero() {
			return
		}
		if !from.IsZero() && s.Time.Before(from) {
			return
		}
		if !to.IsZero() && s.Time.After(to) {
			return
		}
		out = append(out, s)
	}
	if !r.full {
		for i := 0; i < r.head; i++ {
			read(i)
		}
		return out
	}
	// Full ring — start from r.head (oldest) and wrap.
	for i := 0; i < r.cap; i++ {
		read((r.head + i) % r.cap)
	}
	return out
}
