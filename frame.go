package canbus

import "fmt"

const (
	standardIdentifierMaximum = uint32(0x7ff)
	extendedIdentifierMaximum = uint32(0x1fffffff)

	// MaxClassicDataLength is the largest valid classic CAN payload.
	MaxClassicDataLength = 8
)

// Identifier is an immutable standard 11-bit or extended 29-bit CAN identifier.
type Identifier struct {
	value    uint32
	extended bool
}

// NewStandardID validates and returns a standard 11-bit identifier.
func NewStandardID(value uint32) (Identifier, error) {
	if value > standardIdentifierMaximum {
		return Identifier{}, fmt.Errorf("%w: standard value %#x exceeds %#x", ErrInvalidIdentifier, value, standardIdentifierMaximum)
	}
	return Identifier{value: value}, nil
}

// NewExtendedID validates and returns an extended 29-bit identifier.
func NewExtendedID(value uint32) (Identifier, error) {
	if value > extendedIdentifierMaximum {
		return Identifier{}, fmt.Errorf("%w: extended value %#x exceeds %#x", ErrInvalidIdentifier, value, extendedIdentifierMaximum)
	}
	return Identifier{value: value, extended: true}, nil
}

// Value returns the identifier bits without SocketCAN flags.
func (id Identifier) Value() uint32 {
	return id.value
}

// Extended reports whether this is an extended 29-bit identifier.
func (id Identifier) Extended() bool {
	return id.extended
}

func (id Identifier) validate() error {
	maximum := standardIdentifierMaximum
	format := "standard"
	if id.extended {
		maximum = extendedIdentifierMaximum
		format = "extended"
	}
	if id.value > maximum {
		return fmt.Errorf("%w: %s value %#x exceeds %#x", ErrInvalidIdentifier, format, id.value, maximum)
	}
	return nil
}

// Frame is an immutable classic CAN frame.
type Frame struct {
	id   Identifier
	data [MaxClassicDataLength]byte
	dlc  uint8
}

// NewFrame validates and copies a classic CAN payload.
func NewFrame(id Identifier, data []byte) (Frame, error) {
	if err := id.validate(); err != nil {
		return Frame{}, err
	}
	if len(data) > MaxClassicDataLength {
		return Frame{}, fmt.Errorf("%w: %d exceeds %d", ErrInvalidDLC, len(data), MaxClassicDataLength)
	}
	frame := Frame{id: id, dlc: uint8(len(data))}
	copy(frame.data[:], data)
	return frame, nil
}

// ID returns the immutable identifier.
func (frame Frame) ID() Identifier {
	return frame.id
}

// DLC returns the classic CAN data length code in the range 0..8.
func (frame Frame) DLC() uint8 {
	return frame.dlc
}

// Data returns a copy of the payload limited to DLC bytes.
func (frame Frame) Data() []byte {
	data := make([]byte, frame.dlc)
	copy(data, frame.data[:frame.dlc])
	return data
}

// Bytes returns the fixed-width payload storage by value.
func (frame Frame) Bytes() [MaxClassicDataLength]byte {
	return frame.data
}
