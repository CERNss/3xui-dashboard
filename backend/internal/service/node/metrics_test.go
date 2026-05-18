package node

import (
	"testing"
	"time"
)

func TestMetricsStore_AppendAndRaw(t *testing.T) {
	m := NewMetricsStore(4)
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		m.Append(1, base.Add(time.Duration(i)*time.Second), float64(i), float64(i*10))
	}
	raw := m.Raw(1, time.Time{}, time.Time{})
	if len(raw) != 3 {
		t.Fatalf("len = %d, want 3", len(raw))
	}
	for i, s := range raw {
		if s.CPU != float64(i) {
			t.Errorf("sample[%d].CPU = %v, want %d", i, s.CPU, i)
		}
	}
}

func TestMetricsStore_DropsOldestWhenFull(t *testing.T) {
	m := NewMetricsStore(3)
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		m.Append(7, base.Add(time.Duration(i)*time.Second), float64(i), 0)
	}
	raw := m.Raw(7, time.Time{}, time.Time{})
	if len(raw) != 3 {
		t.Fatalf("len = %d, want 3", len(raw))
	}
	if raw[0].CPU != 2 || raw[2].CPU != 4 {
		t.Errorf("oldest dropped wrong: %+v", raw)
	}
}

func TestMetricsStore_RawFilteredByWindow(t *testing.T) {
	m := NewMetricsStore(10)
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		m.Append(1, base.Add(time.Duration(i)*time.Minute), float64(i), 0)
	}
	got := m.Raw(1, base.Add(2*time.Minute), base.Add(4*time.Minute))
	if len(got) != 3 {
		t.Fatalf("window len = %d, want 3 (minutes 2,3,4)", len(got))
	}
	if got[0].CPU != 2 || got[2].CPU != 4 {
		t.Errorf("window bounds wrong: %+v", got)
	}
}

func TestMetricsStore_BucketAverages(t *testing.T) {
	m := NewMetricsStore(20)
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	// 6 samples, 1 minute apart, alternating CPU values.
	for i := 0; i < 6; i++ {
		m.Append(2, base.Add(time.Duration(i)*time.Minute), float64(i*10), 0)
	}
	// 5-minute buckets → samples at minutes 0..4 land in bucket 0
	// (avg of 0,10,20,30,40 = 20), sample at minute 5 is bucket 1
	// (avg of 50 = 50).
	got := m.Bucketed(2, time.Time{}, time.Time{}, 5*time.Minute)
	if len(got) != 2 {
		t.Fatalf("bucketed len = %d, want 2", len(got))
	}
	if got[0].CPU != 20 || got[1].CPU != 50 {
		t.Errorf("bucket averages wrong: %+v", got)
	}
}

func TestMetricsStore_DropForgetsNode(t *testing.T) {
	m := NewMetricsStore(4)
	m.Append(99, time.Now(), 1, 1)
	m.Drop(99)
	if got := m.Raw(99, time.Time{}, time.Time{}); got != nil {
		t.Errorf("after Drop: got %+v, want nil", got)
	}
}
