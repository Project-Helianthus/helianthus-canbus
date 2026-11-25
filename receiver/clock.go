package receiver

import "time"

// clockOrigin is the one receive clock shared by every sequenced receiver,
// including across a reopen.
var clockOrigin = time.Now()

// MonotonicAt converts an instant to elapsed time on the receive clock.
func MonotonicAt(at time.Time) time.Duration {
	return at.Sub(clockOrigin)
}

func monotonicNow() time.Duration {
	return time.Since(clockOrigin)
}
