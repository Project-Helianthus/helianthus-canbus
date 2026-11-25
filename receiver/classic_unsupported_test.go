//go:build !linux

package receiver

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/Project-Helianthus/helianthus-canbus"
)

func TestClassicUnsupportedPlatformFailsClosed(t *testing.T) {
	receiver, err := OpenClassic("can7")
	want := fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
	if !errors.Is(err, canbus.ErrUnsupportedPlatform) || err.Error() != want.Error() || receiver != nil {
		t.Fatalf("receiver = %v, err = %v", receiver, err)
	}
	rejected, err := OpenClassic("")
	if !errors.Is(err, canbus.ErrInvalidInterface) || err.Error() != canbus.ErrInvalidInterface.Error() || rejected != nil {
		t.Fatalf("receiver = %v, err = %v", rejected, err)
	}
}
