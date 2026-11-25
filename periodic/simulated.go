package periodic

import (
	"sync"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
)

// Sent is one frame recorded by a simulated bus, with the clock reading taken
// when the local write completed.
type Sent struct {
	Frame     canbus.Frame
	Monotonic time.Duration
}

// Simulated is an in-process bus. It records each accepted frame and does not
// open a platform interface.
type Simulated struct {
	mu     sync.Mutex
	now    func() time.Duration
	sent   []Sent
	fail   error
	before func()
}

// NewSimulated returns a bus that stamps receipts with now.
func NewSimulated(now func() time.Duration) *Simulated {
	return &Simulated{now: now}
}

// Submit records one frame. The receipt is taken immediately after the local
// record is stored and before Submit returns.
func (s *Simulated) Submit(frame canbus.Frame) (Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		err := s.fail
		s.fail = nil
		return Submission{}, err
	}
	if s.before != nil {
		s.before()
		s.before = nil
	}
	sub, err := accepted(s.now, func() error { return nil })
	if err != nil || !sub.Accepted {
		return sub, err
	}
	s.sent = append(s.sent, Sent{Frame: frame, Monotonic: sub.Monotonic})
	return sub, nil
}

// Frames returns a copy of the recorded frames in submission order.
func (s *Simulated) Frames() []Sent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Sent, len(s.sent))
	copy(out, s.sent)
	return out
}
