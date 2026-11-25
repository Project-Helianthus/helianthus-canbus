//go:build linux

package receiver_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/Project-Helianthus/helianthus-canbus/receiver"
	"golang.org/x/sys/unix"
)

type trace struct{ sockets, binds, closes int }

func scripted(t *trace, lookup func(string) (*net.Interface, error)) receiver.Syscalls {
	return receiver.Syscalls{
		LookupInterface: lookup,
		OpenRaw:         func(int, int, int) (int, error) { t.sockets++; return 5, nil },
		Bind:            func(int, unix.Sockaddr) error { t.binds++; return nil },
		Poll: func(fds []unix.PollFd, _ int) (int, error) {
			fds[0].Revents = unix.POLLIN
			return 1, nil
		},
		Read:  func(_ int, b []byte) (int, error) { b[4] = 8; return 16, nil },
		Close: func(int) error { t.closes++; return nil },
	}
}

func found(name string, index int) func(string) (*net.Interface, error) {
	return func(string) (*net.Interface, error) { return &net.Interface{Name: name, Index: index}, nil }
}

func TestOpenWithReceivedSequence(t *testing.T) {
	var tr trace
	r, err := receiver.OpenWith("synthetic0", scripted(&tr, found("synthetic0", 3)))
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Sequence != 1 || second.Sequence != 2 || len(first.Raw) != 16 ||
		first.Interface.Name() != "synthetic0" || second.Monotonic < first.Monotonic {
		t.Fatalf("records %+v %+v", first, second)
	}
	if err := r.Close(); err != nil || tr.closes != 1 {
		t.Fatal("close", err, tr.closes)
	}
}

func TestOpenWithIdentityMismatch(t *testing.T) {
	var tr trace
	r, err := receiver.OpenWith("synthetic0", scripted(&tr, found("other0", 3)))
	if err == nil || r != nil || tr.sockets != 0 || tr.binds != 0 {
		t.Fatal("mismatch must fail before a socket", err, tr)
	}
}

func TestOpenWithMissingInterface(t *testing.T) {
	var tr trace
	missing := errors.New("no such interface")
	r, err := receiver.OpenWith("synthetic0", scripted(&tr, func(string) (*net.Interface, error) { return nil, missing }))
	if !errors.Is(err, missing) || r != nil || tr.sockets != 0 || tr.binds != 0 {
		t.Fatal("missing interface", err, tr)
	}
}

func TestOpenWithReopenCarriesClock(t *testing.T) {
	var tr trace
	ops := scripted(&tr, found("synthetic0", 3))
	a, err := receiver.OpenWith("synthetic0", ops)
	if err != nil {
		t.Fatal(err)
	}
	ra, err := a.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := receiver.OpenWith("synthetic0", ops)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := b.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rb.Sequence != 1 || rb.Monotonic < ra.Monotonic {
		t.Fatalf("clock restarted: %v then %v", ra.Monotonic, rb.Monotonic)
	}
}
