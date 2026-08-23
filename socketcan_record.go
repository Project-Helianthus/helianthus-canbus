package canbus

import (
	"encoding/binary"
	"fmt"
)

const (
	// SocketCANRecordSize is the byte size of Linux struct can_frame.
	SocketCANRecordSize = 16
	// SocketCANFDRecordSize is recognized only so FD input can fail explicitly.
	SocketCANFDRecordSize = 72

	socketCANExtendedFlag = uint32(0x80000000)
	socketCANRTRFlag      = uint32(0x40000000)
	socketCANErrorFlag    = uint32(0x20000000)
)

// DecodeSocketCANRecord validates and decodes one native-endian struct can_frame.
func DecodeSocketCANRecord(record []byte) (Frame, error) {
	frame, _, err := decodeSocketCANRecord(record)
	return frame, err
}

func decodeSocketCANRecord(record []byte) (Frame, [SocketCANRecordSize]byte, error) {
	var raw [SocketCANRecordSize]byte
	if len(record) == SocketCANFDRecordSize {
		return Frame{}, raw, fmt.Errorf("%w: got %d bytes", ErrUnsupportedFD, len(record))
	}
	if len(record) != SocketCANRecordSize {
		return Frame{}, raw, fmt.Errorf("%w: got %d, want %d", ErrInvalidRecordLength, len(record), SocketCANRecordSize)
	}
	copy(raw[:], record)

	rawIdentifier := binary.NativeEndian.Uint32(record[:4])
	if rawIdentifier&socketCANRTRFlag != 0 {
		return Frame{}, raw, ErrUnsupportedRTR
	}
	if rawIdentifier&socketCANErrorFlag != 0 {
		return Frame{}, raw, ErrUnsupportedErrorFrame
	}

	var (
		identifier Identifier
		err        error
	)
	if rawIdentifier&socketCANExtendedFlag != 0 {
		identifier, err = NewExtendedID(rawIdentifier & extendedIdentifierMaximum)
	} else {
		if rawIdentifier&^standardIdentifierMaximum != 0 {
			return Frame{}, raw, fmt.Errorf("%w: unflagged value %#x exceeds %#x", ErrInvalidIdentifier, rawIdentifier, standardIdentifierMaximum)
		}
		identifier, err = NewStandardID(rawIdentifier)
	}
	if err != nil {
		return Frame{}, raw, err
	}

	dlc := int(record[4])
	if dlc > MaxClassicDataLength {
		return Frame{}, raw, fmt.Errorf("%w: %d exceeds %d", ErrInvalidDLC, dlc, MaxClassicDataLength)
	}
	frame, err := NewFrame(identifier, record[8:8+dlc])
	if err != nil {
		return Frame{}, raw, err
	}
	return frame, raw, nil
}
