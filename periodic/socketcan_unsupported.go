//go:build !linux

package periodic

import (
	"fmt"
	"runtime"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
)

// SocketCAN is a raw SocketCAN writer bound to one interface.
// On this OS the open returns ErrUnsupportedPlatform.
type SocketCAN struct{}

// OpenSocketCAN reports that the raw writer is unavailable on this OS.
// The name is not validated.
func OpenSocketCAN(string, func() time.Duration) (*SocketCAN, error) {
	return nil, fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
}

// Submit reports the same unsupported-platform result as OpenSocketCAN.
func (s *SocketCAN) Submit(canbus.Frame) (Submission, error) {
	return Submission{}, fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
}

// Close is a no-op where no socket was opened.
func (s *SocketCAN) Close() error { return nil }
