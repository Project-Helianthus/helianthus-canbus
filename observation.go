package canbus

import (
	"fmt"
	"strings"
	"time"
)

const maximumInterfaceNameLength = 15

// InterfaceIdentity identifies the Linux network interface that produced evidence.
type InterfaceIdentity struct {
	name  string
	index int
}

// NewInterfaceIdentity validates and returns immutable interface identity.
func NewInterfaceIdentity(name string, index int) (InterfaceIdentity, error) {
	if name == "" || len(name) > maximumInterfaceNameLength || strings.IndexByte(name, 0) >= 0 || index <= 0 {
		return InterfaceIdentity{}, fmt.Errorf("%w: name length=%d index=%d", ErrInvalidInterface, len(name), index)
	}
	return InterfaceIdentity{name: name, index: index}, nil
}

// Name returns the interface name captured when the endpoint opened.
func (identity InterfaceIdentity) Name() string {
	return identity.name
}

// Index returns the interface index captured when the endpoint opened.
func (identity InterfaceIdentity) Index() int {
	return identity.index
}

func (identity InterfaceIdentity) validate() error {
	_, err := NewInterfaceIdentity(identity.name, identity.index)
	return err
}

// Observation is immutable bounded evidence for one accepted classic CAN frame.
type Observation struct {
	frame     Frame
	identity  InterfaceIdentity
	sequence  uint64
	monotonic time.Duration
	rawRecord [SocketCANRecordSize]byte
}

// Frame returns the immutable decoded frame by value.
func (observation Observation) Frame() Frame {
	return observation.frame
}

// Interface returns the immutable source interface identity.
func (observation Observation) Interface() InterfaceIdentity {
	return observation.identity
}

// Sequence returns the listener-local sequence, beginning at one.
func (observation Observation) Sequence() uint64 {
	return observation.sequence
}

// MonotonicTimestamp returns nondecreasing elapsed time since listener creation.
func (observation Observation) MonotonicTimestamp() time.Duration {
	return observation.monotonic
}

// RawRecord returns the exact fixed-size SocketCAN record by value.
func (observation Observation) RawRecord() [SocketCANRecordSize]byte {
	return observation.rawRecord
}
