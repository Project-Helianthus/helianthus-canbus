package canbus

import (
	"errors"
	"testing"
)

func TestIdentifierBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    uint32
		extended bool
		wantErr  error
	}{
		{name: "standard zero", value: 0},
		{name: "standard maximum", value: 0x7ff},
		{name: "standard overflow", value: 0x800, wantErr: ErrInvalidIdentifier},
		{name: "extended zero", value: 0, extended: true},
		{name: "extended maximum", value: 0x1fffffff, extended: true},
		{name: "extended overflow", value: 0x20000000, extended: true, wantErr: ErrInvalidIdentifier},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				got Identifier
				err error
			)
			if tt.extended {
				got, err = NewExtendedID(tt.value)
			} else {
				got, err = NewStandardID(tt.value)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Value() != tt.value {
				t.Fatalf("value = %#x, want %#x", got.Value(), tt.value)
			}
			if got.Extended() != tt.extended {
				t.Fatalf("extended = %t, want %t", got.Extended(), tt.extended)
			}
		})
	}
}

func TestFrameDLCBoundariesAndImmutability(t *testing.T) {
	t.Parallel()

	id, err := NewStandardID(0x321)
	if err != nil {
		t.Fatal(err)
	}

	for dlc := 0; dlc <= MaxClassicDataLength; dlc++ {
		t.Run(string(rune('0'+dlc)), func(t *testing.T) {
			input := make([]byte, dlc)
			for i := range input {
				input[i] = byte(i + 1)
			}
			frame, err := NewFrame(id, input)
			if err != nil {
				t.Fatalf("NewFrame() error = %v", err)
			}
			if frame.DLC() != uint8(dlc) {
				t.Fatalf("DLC = %d, want %d", frame.DLC(), dlc)
			}
			if frame.PayloadLength() != uint8(dlc) {
				t.Fatalf("PayloadLength = %d, want %d", frame.PayloadLength(), dlc)
			}
			if frame.RawDLC() != uint8(dlc) {
				t.Fatalf("RawDLC = %d, want %d", frame.RawDLC(), dlc)
			}
			if frame.ID() != id {
				t.Fatalf("ID = %#v, want %#v", frame.ID(), id)
			}

			if dlc > 0 {
				input[0] ^= 0xff
				got := frame.Data()
				if got[0] != 1 {
					t.Fatalf("frame aliased constructor input: %x", got)
				}
				got[0] ^= 0xff
				if frame.Data()[0] != 1 {
					t.Fatalf("frame aliased returned data: %x", frame.Data())
				}
			}

			fixed := frame.Bytes()
			for i := dlc; i < len(fixed); i++ {
				if fixed[i] != 0 {
					t.Fatalf("byte %d beyond DLC = %#x, want zero", i, fixed[i])
				}
			}
		})
	}

	if _, err := NewFrame(id, make([]byte, MaxClassicDataLength+1)); !errors.Is(err, ErrInvalidDLC) {
		t.Fatalf("oversized payload error = %v, want %v", err, ErrInvalidDLC)
	}
}
