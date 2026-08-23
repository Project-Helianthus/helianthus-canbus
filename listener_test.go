package canbus

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeBackendRead struct {
	record []byte
	err    error
}

type fakeReceiveBackend struct {
	identity   InterfaceIdentity
	reads      chan fakeBackendRead
	closed     chan struct{}
	closeOnce  sync.Once
	closeCalls atomic.Uint32
	closeErr   error
}

func newFakeReceiveBackend(t *testing.T) *fakeReceiveBackend {
	t.Helper()
	identity, err := NewInterfaceIdentity("test-can0", 17)
	if err != nil {
		t.Fatalf("NewInterfaceIdentity() error = %v", err)
	}
	return &fakeReceiveBackend{
		identity: identity,
		reads:    make(chan fakeBackendRead, 32),
		closed:   make(chan struct{}),
	}
}

func (b *fakeReceiveBackend) Identity() InterfaceIdentity {
	return b.identity
}

func (b *fakeReceiveBackend) Read(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result, ok := <-b.reads:
		if !ok {
			return nil, io.EOF
		}
		return append([]byte(nil), result.record...), result.err
	}
}

func (b *fakeReceiveBackend) Close() error {
	b.closeCalls.Add(1)
	b.closeOnce.Do(func() { close(b.closed) })
	return b.closeErr
}

type scriptedClock struct {
	mu     sync.Mutex
	values []time.Time
	last   time.Time
}

func (c *scriptedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.values) == 0 {
		return c.last
	}
	c.last = c.values[0]
	c.values = c.values[1:]
	return c.last
}

func TestListenerBoundedQueueDropNewest(t *testing.T) {
	backend := newFakeReceiveBackend(t)
	for i := 0; i < 4; i++ {
		backend.reads <- fakeBackendRead{record: socketCANFixture(uint32(0x100+i), 1, byte(i))}
	}

	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 2, OverflowPolicy: DropNewest}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	waitForStats(t, listener, func(stats ListenerStats) bool { return stats.Received == 4 })
	if got := listener.Stats(); got.Dropped != 2 || got.Enqueued != 2 {
		t.Fatalf("stats = %+v, want dropped=2 enqueued=2", got)
	}

	first := receiveObservation(t, listener)
	second := receiveObservation(t, listener)
	if first.Sequence() != 1 || second.Sequence() != 2 {
		t.Fatalf("sequences = (%d, %d), want (1, 2)", first.Sequence(), second.Sequence())
	}
}

func TestListenerBoundedQueueDropOldest(t *testing.T) {
	backend := newFakeReceiveBackend(t)
	for i := 0; i < 4; i++ {
		backend.reads <- fakeBackendRead{record: socketCANFixture(uint32(0x200+i), 1, byte(i))}
	}

	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 2, OverflowPolicy: DropOldest}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	waitForStats(t, listener, func(stats ListenerStats) bool { return stats.Received == 4 })
	if got := listener.Stats(); got.Dropped != 2 || got.Enqueued != 4 {
		t.Fatalf("stats = %+v, want dropped=2 enqueued=4", got)
	}

	first := receiveObservation(t, listener)
	second := receiveObservation(t, listener)
	if first.Sequence() != 3 || second.Sequence() != 4 {
		t.Fatalf("sequences = (%d, %d), want (3, 4)", first.Sequence(), second.Sequence())
	}
}

func TestListenerObservationProvenanceIsImmutableAndMonotonic(t *testing.T) {
	backend := newFakeReceiveBackend(t)
	firstRecord := socketCANFixture(0x123, 2, 0xaa, 0xbb)
	secondRecord := socketCANFixture(testExtendedFlag|0x12345, 1, 0xcc)
	backend.reads <- fakeBackendRead{record: firstRecord}
	backend.reads <- fakeBackendRead{record: secondRecord}

	base := time.Unix(1_700_000_000, 0)
	clock := &scriptedClock{values: []time.Time{
		base,
		base.Add(10 * time.Millisecond),
		base.Add(5 * time.Millisecond),
	}}
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 4, OverflowPolicy: DropNewest}, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	waitForStats(t, listener, func(stats ListenerStats) bool { return stats.Received == 2 })
	first := receiveObservation(t, listener)
	second := receiveObservation(t, listener)

	if first.Sequence() != 1 || second.Sequence() != 2 {
		t.Fatalf("sequences = (%d, %d), want (1, 2)", first.Sequence(), second.Sequence())
	}
	if first.MonotonicTimestamp() != 10*time.Millisecond {
		t.Fatalf("first timestamp = %s, want 10ms", first.MonotonicTimestamp())
	}
	if second.MonotonicTimestamp() < first.MonotonicTimestamp() {
		t.Fatalf("timestamps regressed: first=%s second=%s", first.MonotonicTimestamp(), second.MonotonicTimestamp())
	}
	if first.Interface().Name() != "test-can0" || first.Interface().Index() != 17 {
		t.Fatalf("interface = (%q, %d)", first.Interface().Name(), first.Interface().Index())
	}
	if got := first.RawRecord(); !bytesEqual(got[:], firstRecord) {
		t.Fatalf("raw record = %x, want %x", got, firstRecord)
	}
	rawCopy := first.RawRecord()
	rawCopy[8] ^= 0xff
	if first.RawRecord()[8] != 0xaa {
		t.Fatal("observation raw record was mutable through returned copy")
	}
	frameCopy := first.Frame().Data()
	frameCopy[0] ^= 0xff
	if first.Frame().Data()[0] != 0xaa {
		t.Fatal("observation frame was mutable through returned copy")
	}
}

func TestListenerCancellationAndCloseAreDeterministic(t *testing.T) {
	backend := newFakeReceiveBackend(t)
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
	if err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := listener.Receive(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Receive(canceled) error = %v, want %v", err, context.Canceled)
	}

	receiveResult := make(chan error, 1)
	go func() {
		_, err := listener.Receive(context.Background())
		receiveResult <- err
	}()

	if err := listener.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case err := <-receiveResult:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("blocked Receive error = %v, want %v", err, ErrClosed)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked Receive did not return after Close")
	}
	select {
	case <-backend.closed:
	case <-time.After(time.Second):
		t.Fatal("backend was not closed")
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if got := backend.closeCalls.Load(); got != 1 {
		t.Fatalf("backend Close calls = %d, want 1", got)
	}
	if _, err := listener.Receive(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Receive after Close error = %v, want %v", err, ErrClosed)
	}
}

func TestListenerRejectsNilContext(t *testing.T) {
	backend := newFakeReceiveBackend(t)
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if _, err := listener.Receive(nil); !errors.Is(err, ErrNilContext) {
		t.Fatalf("Receive(nil) error = %v, want %v", err, ErrNilContext)
	}
}

func TestListenerPropagatesBackendAndRecordErrors(t *testing.T) {
	t.Run("backend", func(t *testing.T) {
		backend := newFakeReceiveBackend(t)
		backendFailure := errors.New("fixture backend failure")
		backend.reads <- fakeBackendRead{err: backendFailure}
		listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })

		if _, err := listener.Receive(context.Background()); !errors.Is(err, backendFailure) {
			t.Fatalf("Receive() error = %v, want backend failure", err)
		}
	})

	t.Run("unsupported record", func(t *testing.T) {
		backend := newFakeReceiveBackend(t)
		backend.reads <- fakeBackendRead{record: socketCANFixture(testRTRFlag|0x123, 0)}
		listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })

		if _, err := listener.Receive(context.Background()); !errors.Is(err, ErrUnsupportedRTR) {
			t.Fatalf("Receive() error = %v, want %v", err, ErrUnsupportedRTR)
		}
		if got := listener.Stats(); got.Rejected != 1 {
			t.Fatalf("rejected = %d, want 1", got.Rejected)
		}
	})
}

func TestListenerCloseErrorIsStable(t *testing.T) {
	backend := newFakeReceiveBackend(t)
	backend.closeErr = errors.New("fixture close failure")
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); !errors.Is(err, backend.closeErr) {
		t.Fatalf("Close() error = %v, want close failure", err)
	}
	if err := listener.Close(); !errors.Is(err, backend.closeErr) {
		t.Fatalf("second Close() error = %v, want stable close failure", err)
	}
}

func TestInjectedOpenerErrorsAndConfigValidation(t *testing.T) {
	backendFailure := errors.New("fixture open failure")
	called := false
	_, err := listenSocketCAN("test-can0", ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, func(string) (receiveBackend, error) {
		called = true
		return nil, backendFailure
	}, time.Now)
	if !called {
		t.Fatal("injected opener was not called")
	}
	if !errors.Is(err, backendFailure) {
		t.Fatalf("listenSocketCAN() error = %v, want backend failure", err)
	}

	tests := []ListenerConfig{
		{QueueCapacity: 0, OverflowPolicy: DropNewest},
		{QueueCapacity: -1, OverflowPolicy: DropNewest},
		{QueueCapacity: MaxQueueCapacity + 1, OverflowPolicy: DropNewest},
		{QueueCapacity: 1, OverflowPolicy: OverflowPolicy(255)},
	}
	for _, config := range tests {
		openerCalled := false
		_, err := listenSocketCAN("test-can0", config, func(string) (receiveBackend, error) {
			openerCalled = true
			return newFakeReceiveBackend(t), nil
		}, time.Now)
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("config %+v: error = %v, want %v", config, err, ErrInvalidConfig)
		}
		if openerCalled {
			t.Fatalf("config %+v: opener called before validation", config)
		}
	}
}

func receiveObservation(t *testing.T, listener Listener) Observation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	observation, err := listener.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	return observation
}

func waitForStats(t *testing.T, listener Listener, ready func(ListenerStats) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ready(listener.Stats()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("listener stats did not reach expected state: %+v", listener.Stats())
}
