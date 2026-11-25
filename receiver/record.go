package receiver

import (
	"context"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
)

// Record is one raw read from a sequenced receiver: interface identity, raw
// bytes, a per-receiver sequence, and elapsed time on the shared receive clock.
type Record struct {
	Interface canbus.InterfaceIdentity
	Raw       []byte
	Sequence  uint64
	Monotonic time.Duration
}

// Receiver is the sequenced SocketCAN receive path.
type Receiver interface {
	Identity() canbus.InterfaceIdentity
	Receive(context.Context) (Record, error)
	Close() error
}
