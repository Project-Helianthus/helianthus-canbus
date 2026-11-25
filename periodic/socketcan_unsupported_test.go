//go:build !linux

package periodic

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/Project-Helianthus/helianthus-canbus"
)

func TestSocketCANWriterUnsupportedPlatform(t *testing.T) {
	t.Parallel()
	want := fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
	for _, name := range []string{"synthetic0", ""} {
		writer, err := OpenSocketCAN(name, nil)
		if writer != nil || !errors.Is(err, canbus.ErrUnsupportedPlatform) || err.Error() != want.Error() {
			t.Fatalf("OpenSocketCAN(%q) writer=%v err=%v, want %s", name, writer, err, want)
		}
	}
	var writer *SocketCAN
	_, err := writer.Submit(mustFrame(t, false, 1, []byte{1}))
	if !errors.Is(err, canbus.ErrUnsupportedPlatform) || err.Error() != want.Error() {
		t.Fatalf("Submit err=%v, want %s", err, want)
	}
}
