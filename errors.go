package canbus

import "errors"

var (
	// ErrInvalidIdentifier reports an identifier outside its selected format.
	ErrInvalidIdentifier = errors.New("canbus: invalid CAN identifier")
	// ErrInvalidDLC reports a payload length outside classic CAN limits.
	ErrInvalidDLC = errors.New("canbus: invalid classic CAN DLC")
	// ErrInvalidRecordLength reports a record that is not a classic SocketCAN frame.
	ErrInvalidRecordLength = errors.New("canbus: invalid SocketCAN record length")
	// ErrUnsupportedFD identifies a CAN FD record, which phase one does not accept.
	ErrUnsupportedFD = errors.New("canbus: CAN FD is unsupported")
	// ErrUnsupportedRTR identifies a remote-transmission-request record.
	ErrUnsupportedRTR = errors.New("canbus: RTR is unsupported")
	// ErrUnsupportedErrorFrame identifies a SocketCAN error record.
	ErrUnsupportedErrorFrame = errors.New("canbus: error frames are unsupported")
	// ErrInvalidInterface reports missing or invalid interface identity.
	ErrInvalidInterface = errors.New("canbus: invalid interface identity")
	// ErrInvalidConfig reports an unbounded or unsupported listener configuration.
	ErrInvalidConfig = errors.New("canbus: invalid listener configuration")
	// ErrNilContext reports a nil context passed to Receive.
	ErrNilContext = errors.New("canbus: nil receive context")
	// ErrClosed reports an operation attempted after listener shutdown.
	ErrClosed = errors.New("canbus: listener closed")
	// ErrUnsupportedPlatform reports that SocketCAN is unavailable on this OS.
	ErrUnsupportedPlatform = errors.New("canbus: SocketCAN is unsupported on this platform")
)
