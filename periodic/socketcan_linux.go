//go:build linux

package periodic

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
	"github.com/Project-Helianthus/helianthus-canbus/receiver"
	"golang.org/x/sys/unix"
)

// SocketCAN is a raw SocketCAN writer bound to one interface.
type SocketCAN struct {
	now      func() time.Duration
	fd       int
	once     sync.Once
	closeErr error
}

// OpenSocketCAN binds a classic raw socket to name.
// A nil clock uses the shared receive-clock origin.
func OpenSocketCAN(name string, now func() time.Duration) (*SocketCAN, error) {
	if now == nil {
		now = func() time.Duration { return receiver.MonotonicAt(time.Now()) }
	}
	if _, err := canbus.NewInterfaceIdentity(name, 1); err != nil {
		return nil, err
	}
	device, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("lookup interface %q: %w", name, err)
	}
	identity, err := canbus.NewInterfaceIdentity(device.Name, device.Index)
	if err != nil {
		return nil, err
	}
	if identity.Name() != name {
		return nil, canbus.ErrInvalidInterface
	}
	descriptor, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.CAN_RAW)
	if err != nil {
		return nil, fmt.Errorf("create SocketCAN socket: %w", err)
	}
	if err := unix.SetsockoptInt(descriptor, unix.SOL_CAN_RAW, unix.CAN_RAW_FD_FRAMES, 0); err != nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("restrict SocketCAN socket to classic CAN: %w", err)
	}
	if err := unix.Bind(descriptor, &unix.SockaddrCAN{Ifindex: device.Index}); err != nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("bind SocketCAN socket to %q: %w", name, err)
	}
	return &SocketCAN{now: now, fd: descriptor}, nil
}

// Submit writes one classic frame. The receipt is taken when the local write
// completes, before this method returns.
func (s *SocketCAN) Submit(frame canbus.Frame) (Submission, error) {
	if s == nil {
		return Submission{}, fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS)
	}
	raw := recordBytes(frame)
	return accepted(s.now, func() error {
		n, err := unix.Write(s.fd, raw[:])
		if err != nil {
			return err
		}
		if n != len(raw) {
			return fmt.Errorf("periodic: short socket write %d", n)
		}
		return nil
	})
}

// Close releases the socket. A second call returns the first result.
func (s *SocketCAN) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() { s.closeErr = unix.Close(s.fd) })
	return s.closeErr
}
