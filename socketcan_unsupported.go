//go:build !linux

package canbus

import (
	"fmt"
	"runtime"
)

func platformOpenSocketCAN(string) (receiveBackend, error) {
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedPlatform, runtime.GOOS)
}
