//go:build linux

package receiver

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"

	"github.com/Project-Helianthus/helianthus-canbus"
	"golang.org/x/sys/unix"
)

// Replay operands. The scratch overlay redirects the classic opener's
// operating-system calls onto these variables. Ordinary tests leave them nil
// and do not open a host interface.
var (
	replayLookup      func(string) (*net.Interface, error)
	replaySocket      func(int, int, int) (int, error)
	replaySetsockopt  func(int, int, int, int) error
	replayBind        func(int, unix.Sockaddr) error
	replayPoll        func([]unix.PollFd, int) (int, error)
	replayRead        func(int, []byte) (int, error)
	replayClose       func(int) error
	replaySocketCalls int
	replayBindCalls   int
)

func TestMissingInterfaceOpensNothing(t *testing.T) {
	path := os.Getenv("CAN_REPLAY_INPUT")
	if path == "" {
		t.Skip("inert without replay input")
	}
	if !classicReplayArmed() {
		t.Fatal("replay input requires the scratch overlay")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Operands struct {
			ConfiguredName string `json:"configured_name"`
			LookupError    string `json:"lookup_error"`
		} `json:"operands"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	if input.Operands.ConfiguredName == "" || input.Operands.LookupError == "" {
		t.Fatal("replay input is missing the configured name or the lookup failure")
	}
	if !ValidInterfaceName(input.Operands.ConfiguredName) {
		t.Fatal("replay input name failed the name check")
	}
	lookupErr := errors.New(input.Operands.LookupError)
	replaySocketCalls = 0
	replayBindCalls = 0
	replayLookup = func(string) (*net.Interface, error) { return nil, lookupErr }
	replaySocket = func(int, int, int) (int, error) {
		replaySocketCalls++
		return 0, errors.New("socket must not be called")
	}
	replaySetsockopt = func(int, int, int, int) error { return errors.New("setsockopt must not be called") }
	replayBind = func(int, unix.Sockaddr) error {
		replayBindCalls++
		return errors.New("bind must not be called")
	}
	replayPoll = func([]unix.PollFd, int) (int, error) { return 0, errors.New("poll must not be called") }
	replayRead = func(int, []byte) (int, error) { return 0, errors.New("read must not be called") }
	replayClose = func(int) error { return errors.New("close must not be called") }

	opened, err := OpenClassic(input.Operands.ConfiguredName)
	if opened != nil || !errors.Is(err, lookupErr) {
		t.Fatalf("receiver = %v, err = %v", opened, err)
	}
	if errors.Is(err, canbus.ErrInvalidInterface) {
		t.Fatalf("lookup failure returned the identity error: %v", err)
	}
	if replaySocketCalls != 0 || replayBindCalls != 0 {
		t.Fatalf("socket calls = %d, bind calls = %d", replaySocketCalls, replayBindCalls)
	}
}
