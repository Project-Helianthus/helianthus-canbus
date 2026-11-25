//go:build linux

package receiver

import (
	"net"

	"golang.org/x/sys/unix"
)

// Syscalls is the set of operating-system operations one sequenced open runs
// on. Every field is required; a nil field panics when the open reaches it.
// Open supplies the real operations; a caller of OpenWith supplies its own.
type Syscalls struct {
	// LookupInterface resolves the interface name to its identity source.
	LookupInterface func(name string) (*net.Interface, error)
	// OpenRaw creates the raw CAN endpoint and returns its descriptor.
	OpenRaw func(domain, kind, protocol int) (int, error)
	// Bind binds the descriptor to the looked-up interface.
	Bind func(fd int, address unix.Sockaddr) error
	// Poll waits for readability; the receive loop passes a 25 ms timeout.
	Poll func(fds []unix.PollFd, timeoutMillis int) (int, error)
	// Read reads one record into the buffer and returns its length.
	Read func(fd int, buffer []byte) (int, error)
	// Close releases the descriptor; it runs at most once per receiver.
	Close func(fd int) error
}

// OpenWith runs the same sequenced open engine as Open on the supplied
// operations, so a caller can drive it without a CAN interface.
func OpenWith(name string, ops Syscalls) (Receiver, error) {
	return openProject(name, projectSyscalls{
		lookup: ops.LookupInterface,
		socket: ops.OpenRaw,
		bind:   ops.Bind,
		poll:   ops.Poll,
		read:   ops.Read,
		close:  ops.Close,
	})
}
