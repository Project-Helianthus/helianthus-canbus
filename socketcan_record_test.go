package canbus

import (
	"encoding/binary"
	"errors"
	"testing"
)

const (
	testExtendedFlag = uint32(0x80000000)
	testRTRFlag      = uint32(0x40000000)
	testErrorFlag    = uint32(0x20000000)
)

func socketCANFixture(rawID uint32, dlc byte, data ...byte) []byte {
	return socketCANFixtureWithLen8DLC(rawID, dlc, 0, data...)
}

func socketCANFixtureWithLen8DLC(rawID uint32, payloadLength, len8DLC byte, data ...byte) []byte {
	record := make([]byte, SocketCANRecordSize)
	binary.NativeEndian.PutUint32(record[:4], rawID)
	record[4] = payloadLength
	record[7] = len8DLC
	copy(record[8:], data)
	return record
}

func TestDecodeSocketCANRecordStandardAndExtended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rawID      uint32
		dlc        byte
		data       []byte
		wantID     uint32
		wantExtend bool
	}{
		{name: "standard", rawID: 0x456, dlc: 3, data: []byte{0x10, 0x20, 0x30}, wantID: 0x456},
		{name: "extended", rawID: testExtendedFlag | 0x18daf110, dlc: 8, data: []byte{1, 2, 3, 4, 5, 6, 7, 8}, wantID: 0x18daf110, wantExtend: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := DecodeSocketCANRecord(socketCANFixture(tt.rawID, tt.dlc, tt.data...))
			if err != nil {
				t.Fatalf("DecodeSocketCANRecord() error = %v", err)
			}
			if frame.ID().Value() != tt.wantID || frame.ID().Extended() != tt.wantExtend {
				t.Fatalf("ID = (%#x, %t), want (%#x, %t)", frame.ID().Value(), frame.ID().Extended(), tt.wantID, tt.wantExtend)
			}
			if frame.DLC() != tt.dlc {
				t.Fatalf("DLC = %d, want %d", frame.DLC(), tt.dlc)
			}
			if got := frame.Data(); !bytesEqual(got, tt.data) {
				t.Fatalf("data = %x, want %x", got, tt.data)
			}
		})
	}
}

func TestDecodeSocketCANRecordDLCBoundaries(t *testing.T) {
	t.Parallel()

	for dlc := byte(0); dlc <= MaxClassicDataLength; dlc++ {
		payload := []byte{0, 1, 2, 3, 4, 5, 6, 7}
		frame, err := DecodeSocketCANRecord(socketCANFixture(0x123, dlc, payload...))
		if err != nil {
			t.Fatalf("DLC %d: error = %v", dlc, err)
		}
		if frame.DLC() != dlc || len(frame.Data()) != int(dlc) {
			t.Fatalf("DLC %d: frame reports DLC=%d len=%d", dlc, frame.DLC(), len(frame.Data()))
		}
	}
}

func TestDecodeSocketCANRecordPreservesLen8DLC(t *testing.T) {
	t.Parallel()

	for _, rawDLC := range []byte{9, 15} {
		t.Run(string(rune('0'+rawDLC)), func(t *testing.T) {
			frame, err := DecodeSocketCANRecord(socketCANFixtureWithLen8DLC(0x123, 8, rawDLC, 1, 2, 3, 4, 5, 6, 7, 8))
			if err != nil {
				t.Fatalf("DecodeSocketCANRecord() error = %v", err)
			}
			if got := frame.PayloadLength(); got != 8 {
				t.Fatalf("PayloadLength = %d, want 8", got)
			}
			if got := frame.DLC(); got != 8 {
				t.Fatalf("DLC = %d, want payload length 8", got)
			}
			if got := frame.RawDLC(); got != rawDLC {
				t.Fatalf("RawDLC = %d, want %d", got, rawDLC)
			}
		})
	}
}

func TestDecodeSocketCANRecordRejectsUnsupportedAndMalformedRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record []byte
		want   error
	}{
		{name: "empty", record: nil, want: ErrInvalidRecordLength},
		{name: "short", record: make([]byte, SocketCANRecordSize-1), want: ErrInvalidRecordLength},
		{name: "long", record: make([]byte, SocketCANRecordSize+1), want: ErrInvalidRecordLength},
		{name: "fd", record: make([]byte, SocketCANFDRecordSize), want: ErrUnsupportedFD},
		{name: "rtr standard", record: socketCANFixture(testRTRFlag|0x123, 0), want: ErrUnsupportedRTR},
		{name: "rtr extended", record: socketCANFixture(testExtendedFlag|testRTRFlag|0x12345, 0), want: ErrUnsupportedRTR},
		{name: "error frame", record: socketCANFixture(testErrorFlag|0x1, 8), want: ErrUnsupportedErrorFrame},
		{name: "invalid standard identifier", record: socketCANFixture(0x800, 0), want: ErrInvalidIdentifier},
		{name: "invalid dlc", record: socketCANFixture(0x123, MaxClassicDataLength+1), want: ErrInvalidDLC},
		{name: "len8 dlc with short payload", record: socketCANFixtureWithLen8DLC(0x123, 7, 9), want: ErrInvalidDLC},
		{name: "len8 dlc below extended range", record: socketCANFixtureWithLen8DLC(0x123, 8, 1), want: ErrInvalidDLC},
		{name: "len8 dlc repeats payload length", record: socketCANFixtureWithLen8DLC(0x123, 8, 8), want: ErrInvalidDLC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeSocketCANRecord(tt.record); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDecodeSocketCANRecordDoesNotExposeBytesBeyondDLC(t *testing.T) {
	t.Parallel()

	record := socketCANFixture(0x111, 2, 0xaa, 0xbb, 0xcc, 0xdd)
	frame, err := DecodeSocketCANRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if got := frame.Data(); !bytesEqual(got, []byte{0xaa, 0xbb}) {
		t.Fatalf("data = %x, want aabb", got)
	}
	if got := frame.Bytes(); got[2] != 0 || got[3] != 0 {
		t.Fatalf("bytes beyond DLC were retained: %x", got)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
