package receiver

import (
	"context"
	"strings"
)

// InterfaceNameMaximum is the interface name limit, IFNAMSIZ without the
// terminator.
const InterfaceNameMaximum = 15

// ClassicIdentity is the interface a classic-only receiver bound.
type ClassicIdentity struct {
	Name  string
	Index int
}

// ClassicRecord is one raw read. It has no clock.
type ClassicRecord struct {
	Identity ClassicIdentity
	Raw      []byte
}

// ClassicReceiver is the classic-only SocketCAN receive path.
type ClassicReceiver interface {
	Identity() ClassicIdentity
	Receive(context.Context) (ClassicRecord, error)
	Close() error
}

// ValidInterfaceName reports whether name can be opened by the classic-only
// receiver. The check is the same on every platform.
func ValidInterfaceName(name string) bool {
	if name == "" || len(name) > InterfaceNameMaximum || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, "/:\x00 \t\r\n")
}
