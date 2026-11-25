package receiver

import (
	"testing"
	"time"
)

func TestMonotonicAtSharesOrigin(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	if MonotonicAt(base.Add(5*time.Millisecond))-MonotonicAt(base) != 5*time.Millisecond {
		t.Fatal("elapsed conversion is not a fixed origin")
	}
	now := time.Now()
	if MonotonicAt(now) != now.Sub(clockOrigin) {
		t.Fatal("elapsed conversion drifted from the receive origin")
	}
	sampled := monotonicNow()
	later := MonotonicAt(time.Now())
	if later < sampled {
		t.Fatal("receive clock went backwards")
	}
}
