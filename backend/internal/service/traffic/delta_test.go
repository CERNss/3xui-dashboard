package traffic

import (
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

func sample(t time.Time, up, down int64) model.TrafficSample {
	return model.TrafficSample{TakenAt: t, UpCumBytes: up, DownCumBytes: down}
}

func TestSumDeltas_Monotonic(t *testing.T) {
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	samples := []model.TrafficSample{
		sample(base, 100, 200),
		sample(base.Add(time.Minute), 150, 300),
		sample(base.Add(2*time.Minute), 200, 400),
	}
	up, down := SumDeltas(samples)
	if up != 100 {
		t.Errorf("up = %d, want 100", up)
	}
	if down != 200 {
		t.Errorf("down = %d, want 200", down)
	}
}

func TestSumDeltas_CounterResetIsBaseline(t *testing.T) {
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	samples := []model.TrafficSample{
		sample(base, 100, 200),
		sample(base.Add(time.Minute), 500, 600), // up by 400/400
		sample(base.Add(2*time.Minute), 50, 50), // RESET → +50 / +50
		sample(base.Add(3*time.Minute), 70, 80), // +20 / +30
	}
	up, down := SumDeltas(samples)
	if up != 400+50+20 {
		t.Errorf("up = %d, want %d (no negative delta on reset)", up, 400+50+20)
	}
	if down != 400+50+30 {
		t.Errorf("down = %d, want %d", down, 400+50+30)
	}
}

func TestSumDeltas_EmptyOrSingle(t *testing.T) {
	up, down := SumDeltas(nil)
	if up != 0 || down != 0 {
		t.Errorf("nil → 0,0; got %d,%d", up, down)
	}
	one := []model.TrafficSample{sample(time.Now(), 100, 200)}
	up, down = SumDeltas(one)
	if up != 0 || down != 0 {
		t.Errorf("single sample → 0 delta; got %d,%d", up, down)
	}
}

func TestBucketDeltas_GroupsAndSums(t *testing.T) {
	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	// Two 5-min buckets, each with two deltas.
	samples := []model.TrafficSample{
		sample(base, 0, 0),
		sample(base.Add(2*time.Minute), 100, 200),    // bucket 0: +100/+200
		sample(base.Add(4*time.Minute), 300, 400),    // bucket 0: +200/+200
		sample(base.Add(7*time.Minute), 500, 600),    // bucket 1: +200/+200
		sample(base.Add(9*time.Minute), 1000, 1000),  // bucket 1: +500/+400
	}
	pts := BucketDeltas(samples, int64(5*time.Minute))
	if len(pts) != 2 {
		t.Fatalf("buckets = %d, want 2", len(pts))
	}
	if pts[0].Up != 300 || pts[0].Down != 400 {
		t.Errorf("bucket 0: got %+v, want up=300 down=400", pts[0])
	}
	if pts[1].Up != 700 || pts[1].Down != 600 {
		t.Errorf("bucket 1: got %+v, want up=700 down=600", pts[1])
	}
}
