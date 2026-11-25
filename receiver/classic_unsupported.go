//go:build !linux

package receiver

import (
	"fmt"
	"runtime"

	"github.com/Project-Helianthus/helianthus-canbus"
)

// OpenClassic checks the interface name, then reports that classic-only
// SocketCAN receive is unavailable on this OS.
func OpenClassic(name string) (ClassicReceiver, error) {
	if !ValidInterfaceName(name) {
		return nil, canbus.ErrInvalidInterface
	}
	return nil, fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
}
