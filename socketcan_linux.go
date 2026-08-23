//go:build linux

package canbus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const receivePollInterval = 25 * time.Millisecond

type linuxReceiveBackend struct {
	identity   InterfaceIdentity
	descriptor int
	closeOnce  sync.Once
	closeErr   error
}

func platformOpenSocketCAN(interfaceName string) (receiveBackend, error) {
	if interfaceName == "" {
		return nil, ErrInvalidInterface
	}
	device, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("lookup interface %q: %w", interfaceName, err)
	}
	identity, err := NewInterfaceIdentity(device.Name, device.Index)
	if err != nil {
		return nil, err
	}

	descriptor, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.CAN_RAW)
	if err != nil {
		return nil, fmt.Errorf("create SocketCAN receive socket: %w", err)
	}
	if err := unix.SetsockoptInt(descriptor, unix.SOL_CAN_RAW, unix.CAN_RAW_FD_FRAMES, 0); err != nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("restrict SocketCAN receive socket to classic CAN: %w", err)
	}
	if err := unix.Bind(descriptor, &unix.SockaddrCAN{Ifindex: device.Index}); err != nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("bind SocketCAN receive socket to %q: %w", interfaceName, err)
	}
	return &linuxReceiveBackend{identity: identity, descriptor: descriptor}, nil
}

func (backend *linuxReceiveBackend) Identity() InterfaceIdentity {
	return backend.identity
}

func (backend *linuxReceiveBackend) Read(ctx context.Context) ([]byte, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pollDescriptor := []unix.PollFd{{Fd: int32(backend.descriptor), Events: unix.POLLIN}}
		ready, err := unix.Poll(pollDescriptor, pollTimeout(ctx))
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return nil, fmt.Errorf("poll SocketCAN receive socket: %w", err)
		}
		if ready == 0 {
			continue
		}
		events := pollDescriptor[0].Revents
		if events&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 && events&unix.POLLIN == 0 {
			return nil, fmt.Errorf("SocketCAN receive socket poll events %#x", events)
		}
		if events&unix.POLLIN == 0 {
			continue
		}

		var record [SocketCANFDRecordSize]byte
		count, err := unix.Read(backend.descriptor, record[:])
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
				continue
			}
			return nil, fmt.Errorf("read SocketCAN record: %w", err)
		}
		return record[:count], nil
	}
}

func (backend *linuxReceiveBackend) Close() error {
	backend.closeOnce.Do(func() {
		backend.closeErr = unix.Close(backend.descriptor)
	})
	return backend.closeErr
}

func pollTimeout(ctx context.Context) int {
	duration := receivePollInterval
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0
		}
		if remaining < duration {
			duration = remaining
		}
	}
	milliseconds := (duration + time.Millisecond - 1) / time.Millisecond
	return int(milliseconds)
}

var _ receiveBackend = (*linuxReceiveBackend)(nil)
