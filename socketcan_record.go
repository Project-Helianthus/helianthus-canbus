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

	payloadLength := record[4]
	if payloadLength > MaxClassicDataLength {
		return Frame{}, raw, fmt.Errorf("%w: payload length %d exceeds %d", ErrInvalidDLC, payloadLength, MaxClassicDataLength)
	}
	rawDLC, err := socketCANRawDLC(payloadLength, record[7])
	if err != nil {
		return Frame{}, raw, err
	}
	frame, err := newFrame(identifier, record[8:8+int(payloadLength)], rawDLC)
	if err != nil {
		return Frame{}, raw, err
	}
	return frame, raw, nil
}

func socketCANRawDLC(payloadLength, len8DLC byte) (uint8, error) {
	if payloadLength != MaxClassicDataLength {
		if len8DLC != 0 {
			return 0, fmt.Errorf("%w: len8_dlc %d requires payload length %d", ErrInvalidDLC, len8DLC, MaxClassicDataLength)
		}
		return payloadLength, nil
	}
	if len8DLC == 0 {
		return payloadLength, nil
	}
	if len8DLC >= 9 && len8DLC <= 15 {
		return len8DLC, nil
	}
	return 0, fmt.Errorf("%w: len8_dlc %d is invalid for payload length %d", ErrInvalidDLC, len8DLC, payloadLength)
}
