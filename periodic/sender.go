package periodic

import (
	"errors"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
)

// Submission is the local result of one write.
// Accepted is true only after the write completes. Monotonic is the clock
// reading taken immediately at that completion. A failed write leaves
// Accepted false and Monotonic at zero.
type Submission struct {
	Accepted  bool
	Monotonic time.Duration
}

// Sender writes one frame. The periodic publisher and the one-shot command
// use the same Sender for one interface.
type Sender interface {
	Submit(canbus.Frame) (Submission, error)
}

func accepted(now func() time.Duration, write func() error) (Submission, error) {
	if write == nil {
		return Submission{}, errors.New("periodic: nil write")
	}
	if err := write(); err != nil {
		return Submission{}, err
	}
	if now == nil {
		return Submission{}, errors.New("periodic: nil clock")
	}
	return Submission{Accepted: true, Monotonic: now()}, nil
}
