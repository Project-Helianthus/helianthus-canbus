//go:build !linux

package receiver

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/Project-Helianthus/helianthus-canbus"
)

func TestOpenUnsupported(t *testing.T) {
	want := fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS).Error()
	for _, name := range []string{"synthetic0", ""} {
		r, err := Open(name)
		if r != nil || err == nil {
			t.Fatal("unsupported platform opened receiver")
		}
		if !errors.Is(err, canbus.ErrUnsupportedPlatform) || err.Error() != want {
			t.Fatalf("Open(%q) error = %v, want %s", name, err, want)
		}
	}
}
