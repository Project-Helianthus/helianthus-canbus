//go:build linux

package receiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
	"golang.org/x/sys/unix"
)

const classicPollInterval = 25 * time.Millisecond

// classicReplayArmed is false in this tree. The scratch replay overlay replaces
// this file, returns true, and substitutes the operating-system calls below
// with replayLookup, replaySocket, replaySetsockopt, replayBind, replayPoll,
// replayRead, and replayClose.
func classicReplayArmed() bool { return false }

type classicSocket struct {
	identity   ClassicIdentity
	descriptor int
	closeOnce  sync.Once
	closeErr   error
}

// OpenClassic opens a classic-only SocketCAN receiver.
// An invalid name, or a lookup whose index is not positive or whose name
// differs, returns ErrInvalidInterface. The name is checked before a socket
// is created.
func OpenClassic(name string) (ClassicReceiver, error) {
	if !ValidInterfaceName(name) {
		return nil, canbus.ErrInvalidInterface
	}
	device, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("lookup CAN interface: %w", err)
	}
	if device.Index <= 0 || device.Name != name {
		return nil, canbus.ErrInvalidInterface
	}
	descriptor, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.CAN_RAW)
	if err != nil {
		return nil, fmt.Errorf("create SocketCAN receive socket: %w", err)
	}
	if err := unix.SetsockoptInt(descriptor, unix.SOL_CAN_RAW, unix.CAN_RAW_FD_FRAMES, 0); err != nil {
		return nil, errors.Join(fmt.Errorf("restrict SocketCAN receive socket to classic CAN: %w", err), unix.Close(descriptor))
	}
	if err := unix.Bind(descriptor, &unix.SockaddrCAN{Ifindex: device.Index}); err != nil {
		return nil, errors.Join(fmt.Errorf("bind SocketCAN receive socket: %w", err), unix.Close(descriptor))
	}
	return &classicSocket{identity: ClassicIdentity{Name: device.Name, Index: device.Index}, descriptor: descriptor}, nil
}

func (socket *classicSocket) Identity() ClassicIdentity { return socket.identity }

func (socket *classicSocket) Receive(ctx context.Context) (ClassicRecord, error) {
	for {
		if err := ctx.Err(); err != nil {
			return ClassicRecord{}, err
		}
		poll := []unix.PollFd{{Fd: int32(socket.descriptor), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, classicPollTimeout(ctx))
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return ClassicRecord{}, fmt.Errorf("poll SocketCAN receive socket: %w", err)
		}
		if ready == 0 {
			continue
		}
		events := poll[0].Revents
		if events&unix.POLLIN == 0 {
			if events&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
				return ClassicRecord{}, fmt.Errorf("SocketCAN receive socket poll events %#x", events)
			}
			continue
		}
		var record [canbus.SocketCANFDRecordSize]byte
		count, err := unix.Read(socket.descriptor, record[:])
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
				continue
			}
			return ClassicRecord{}, fmt.Errorf("read SocketCAN record: %w", err)
		}
		return ClassicRecord{Identity: socket.identity, Raw: append([]byte(nil), record[:count]...)}, nil
	}
}

func (socket *classicSocket) Close() error {
	socket.closeOnce.Do(func() { socket.closeErr = unix.Close(socket.descriptor) })
	return socket.closeErr
}

func classicPollTimeout(ctx context.Context) int {
	duration := classicPollInterval
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0
		}
		if remaining < duration {
			duration = remaining
		}
	}
	return int((duration + time.Millisecond - 1) / time.Millisecond)
}
