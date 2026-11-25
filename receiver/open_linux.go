//go:build linux

package receiver

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/Project-Helianthus/helianthus-canbus"
	"golang.org/x/sys/unix"
)

// projectSyscalls is the operating-system set for one sequenced open.
// The opener looks up, binds, polls, and reads.
type projectSyscalls struct {
	lookup func(string) (*net.Interface, error)
	socket func(int, int, int) (int, error)
	bind   func(int, unix.Sockaddr) error
	poll   func([]unix.PollFd, int) (int, error)
	read   func(int, []byte) (int, error)
	close  func(int) error
}

type projectSocket struct {
	identity canbus.InterfaceIdentity
	fd       int
	ops      projectSyscalls
	sequence uint64
	once     sync.Once
	closeErr error
}

// Open opens a sequenced SocketCAN receiver on name.
// An invalid name returns ErrInvalidInterface before a socket is created.
func Open(name string) (Receiver, error) {
	return openProject(name, projectSyscalls{
		lookup: net.InterfaceByName,
		socket: unix.Socket,
		bind:   unix.Bind,
		poll:   unix.Poll,
		read:   unix.Read,
		close:  unix.Close,
	})
}

func openProject(name string, ops projectSyscalls) (Receiver, error) {
	if _, err := canbus.NewInterfaceIdentity(name, 1); err != nil {
		return nil, err
	}
	device, err := ops.lookup(name)
	if err != nil {
		return nil, err
	}
	if device == nil || device.Name != name {
		return nil, errors.New("selected interface identity mismatch")
	}
	identity, err := canbus.NewInterfaceIdentity(device.Name, device.Index)
	if err != nil {
		return nil, err
	}
	fd, err := ops.socket(unix.AF_CAN, unix.SOCK_RAW|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, unix.CAN_RAW)
	if err != nil {
		return nil, err
	}
	socket := &projectSocket{identity: identity, fd: fd, ops: ops}
	if err := ops.bind(fd, &unix.SockaddrCAN{Ifindex: device.Index}); err != nil {
		if closeErr := socket.Close(); closeErr != nil {
			return socket, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return socket, nil
}

func (s *projectSocket) Identity() canbus.InterfaceIdentity { return s.identity }

func (s *projectSocket) Receive(ctx context.Context) (Record, error) {
	for {
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		poll := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN}}
		ready, err := s.ops.poll(poll, 25)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return Record{}, err
		}
		if ready == 0 {
			continue
		}
		if poll[0].Revents&unix.POLLIN == 0 {
			return Record{}, errors.New("SocketCAN receive poll failure")
		}
		var raw [canbus.SocketCANFDRecordSize]byte
		n, err := s.ops.read(s.fd, raw[:])
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return Record{}, err
		}
		if n < 0 || n > len(raw) {
			return Record{}, errors.New("invalid SocketCAN read length")
		}
		s.sequence++
		return Record{
			Interface: s.identity,
			Raw:       append([]byte(nil), raw[:n]...),
			Sequence:  s.sequence,
			Monotonic: monotonicNow(),
		}, nil
	}
}

func (s *projectSocket) Close() error {
	s.once.Do(func() { s.closeErr = s.ops.close(s.fd) })
	return s.closeErr
}
