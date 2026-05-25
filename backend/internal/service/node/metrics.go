package node

import (
	"sort"
	"sync"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// MetricSample is one health reading stored in the per-node ring. It is
// populated by the periodic probe job from the upstream panel's
// /server/status response. CPU/Mem are percentages for charting; the
// remaining fields preserve the useful raw health context.
type MetricSample struct {
	Time            time.Time `json:"time"`
	Status          string    `json:"status,omitempty"`
	CPU             float64   `json:"cpu"`
	Mem             float64   `json:"mem"`
	CPUCores        int       `json:"cpu_cores,omitempty"`
	MemCurrentBytes int64     `json:"mem_current_bytes,omitempty"`
	MemTotalBytes   int64     `json:"mem_total_bytes,omitempty"`
	UptimeSecs      int64     `json:"uptime_s,omitempty"`
	Load1           float64   `json:"load1,omitempty"`
	Load5           float64   `json:"load5,omitempty"`
	Load15          float64   `json:"load15,omitempty"`
	NetUpBytes      int64     `json:"net_up_bytes,omitempty"`
	NetDownBytes    int64     `json:"net_down_bytes,omitempty"`
	XrayState       string    `json:"xray_state,omitempty"`
	XrayError       string    `json:"xray_error,omitempty"`
	XrayVersion     string    `json:"xray_version,omitempty"`
	PublicIPv4      string    `json:"public_ipv4,omitempty"`
	PublicIPv6      string    `json:"public_ipv6,omitempty"`
	Error           string    `json:"error,omitempty"`
}

// MetricsStore is the in-memory per-node ring buffer. With the default 60s
// probe cadence the cap of 720 samples covers ~12 hours before the retention
// window trims it down for the admin charts.
type MetricsStore struct {
	mu     sync.RWMutex
	cap    int
	window time.Duration
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

// SetRetention keeps future reads and writes bounded to the configured
// history window. Passing 0 disables time-based trimming and leaves only the
// sample-count cap in effect.
func (m *MetricsStore) SetRetention(window time.Duration) {
	m.mu.Lock()
	if window < 0 {
		window = 0
	}
	m.window = window
	cutoff := retentionCutoff(time.Now().UTC(), window)
	if !cutoff.IsZero() {
		for _, r := range m.tables {
			r.trimBefore(cutoff)
		}
	}
	m.mu.Unlock()
}

// Append records one sample for nodeID. Drops the oldest entry once
// the ring is full.
func (m *MetricsStore) Append(nodeID int64, ts time.Time, cpu, mem float64) {
	m.appendSample(nodeID, MetricSample{Time: ts, CPU: cpu, Mem: mem})
}

// AppendStatus records a successful panel health sample.
func (m *MetricsStore) AppendStatus(nodeID int64, ts time.Time, status *runtime.Status) {
	if status == nil {
		return
	}
	loads := [3]float64{}
	for i := 0; i < len(loads) && i < len(status.Loads); i++ {
		loads[i] = status.Loads[i]
	}
	m.appendSample(nodeID, MetricSample{
		Time:            ts,
		Status:          model.NodeStatusOnline,
		CPU:             status.CPU,
		Mem:             status.MemPercent(),
		CPUCores:        status.CPUCores,
		MemCurrentBytes: status.Mem.Current,
		MemTotalBytes:   status.Mem.Total,
		UptimeSecs:      status.Uptime,
		Load1:           loads[0],
		Load5:           loads[1],
		Load15:          loads[2],
		NetUpBytes:      status.NetIO.Up,
		NetDownBytes:    status.NetIO.Down,
		XrayState:       status.Xray.State,
		XrayError:       status.Xray.ErrorMsg,
		XrayVersion:     status.Xray.Version,
		PublicIPv4:      status.PublicIP.IPv4,
		PublicIPv6:      status.PublicIP.IPv6,
	})
}

// AppendFailure records a failed collection attempt. Keeping failures in
// the same ring lets ops views distinguish "no data yet" from "collector
// ran and the panel was unreachable".
func (m *MetricsStore) AppendFailure(nodeID int64, ts time.Time, err error) {
	sample := MetricSample{Time: ts, Status: model.NodeStatusOffline}
	if err != nil {
		sample.Error = err.Error()
	}
	m.appendSample(nodeID, sample)
}

func (m *MetricsStore) appendSample(nodeID int64, sample MetricSample) {
	m.mu.Lock()
	r, ok := m.tables[nodeID]
	if !ok {
		r = newRing(m.cap)
		m.tables[nodeID] = r
	}
	r.push(sample)
	if cutoff := retentionCutoff(sample.Time, m.window); !cutoff.IsZero() {
		r.trimBefore(cutoff)
	}
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
	defer m.mu.RUnlock()
	r, ok := m.tables[nodeID]
	window := m.window
	if !ok {
		return nil
	}
	if cutoff := retentionCutoff(time.Now().UTC(), window); !cutoff.IsZero() && (from.IsZero() || from.Before(cutoff)) {
		from = cutoff
	}
	return r.window(from, to)
}

func retentionCutoff(now time.Time, window time.Duration) time.Time {
	if window <= 0 || now.IsZero() {
		return time.Time{}
	}
	return now.Add(-window)
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
		cpu    float64
		mem    float64
		load1  float64
		load5  float64
		load15 float64
		n      int
		last   MetricSample
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
		a.load1 += s.Load1
		a.load5 += s.Load5
		a.load15 += s.Load15
		a.n++
		a.last = s
	}

	out := make([]MetricSample, 0, len(buckets))
	for k, a := range buckets {
		sample := a.last
		sample.Time = time.Unix(0, k*int64(bucket)).UTC()
		sample.CPU = a.cpu / float64(a.n)
		sample.Mem = a.mem / float64(a.n)
		sample.Load1 = a.load1 / float64(a.n)
		sample.Load5 = a.load5 / float64(a.n)
		sample.Load15 = a.load15 / float64(a.n)
		out = append(out, sample)
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

func (r *ring) trimBefore(cutoff time.Time) {
	if cutoff.IsZero() {
		return
	}
	for i := range r.data {
		s := r.data[i]
		if !s.Time.IsZero() && s.Time.Before(cutoff) {
			r.data[i] = MetricSample{}
		}
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
