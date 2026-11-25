//go:build !linux

package receiver

import (
	"fmt"
	"runtime"

	"github.com/Project-Helianthus/helianthus-canbus"
)

// Open reports that sequenced SocketCAN receive is unavailable on this OS.
// The name is not validated.
func Open(string) (Receiver, error) {
	return nil, fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
}
