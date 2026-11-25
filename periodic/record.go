package periodic

import (
	"encoding/binary"

	"github.com/Project-Helianthus/helianthus-canbus"
)

const extendedFlag = uint32(0x80000000)

func recordBytes(frame canbus.Frame) [canbus.SocketCANRecordSize]byte {
	var raw [canbus.SocketCANRecordSize]byte
	identifier := frame.ID().Value()
	if frame.ID().Extended() {
		identifier |= extendedFlag
	}
	binary.NativeEndian.PutUint32(raw[:4], identifier)
	raw[4] = frame.DLC()
	if frame.RawDLC() > canbus.MaxClassicDataLength {
		raw[7] = frame.RawDLC()
	}
	payload := frame.Bytes()
	copy(raw[8:], payload[:frame.DLC()])
	return raw
}
