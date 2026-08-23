package canbus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxQueueCapacity prevents configurations from allocating an unbounded queue.
	MaxQueueCapacity = 65_536

	listenerClosed = uint32(1)
)

// OverflowPolicy defines deterministic behavior when the receive queue is full.
type OverflowPolicy uint8

const (
	// DropNewest preserves queued evidence and drops the newly received observation.
	DropNewest OverflowPolicy = iota + 1
	// DropOldest drops the oldest queued observation and retains the newest one.
	DropOldest
)

// ListenerConfig configures an explicitly bounded receive queue.
type ListenerConfig struct {
	QueueCapacity  int
	OverflowPolicy OverflowPolicy
}

func (config ListenerConfig) validate() error {
	if config.QueueCapacity <= 0 || config.QueueCapacity > MaxQueueCapacity {
		return fmt.Errorf("%w: queue capacity %d is outside 1..%d", ErrInvalidConfig, config.QueueCapacity, MaxQueueCapacity)
	}
	if config.OverflowPolicy != DropNewest && config.OverflowPolicy != DropOldest {
		return fmt.Errorf("%w: overflow policy %d", ErrInvalidConfig, config.OverflowPolicy)
	}
	return nil
}

// ListenerStats is a race-safe snapshot of listener counters.
type ListenerStats struct {
	Received  uint64
	Enqueued  uint64
	Delivered uint64
	Dropped   uint64
	Rejected  uint64
}

// Listener exposes only receive, statistics, and lifecycle operations.
type Listener interface {
	Receive(context.Context) (Observation, error)
	Stats() ListenerStats
	Close() error
}

type receiveBackend interface {
	Identity() InterfaceIdentity
	Read(context.Context) ([]byte, error)
	Close() error
}

type listenerCounters struct {
	received  atomic.Uint64
	enqueued  atomic.Uint64
	delivered atomic.Uint64
	dropped   atomic.Uint64
	rejected  atomic.Uint64
}

type listener struct {
	backend  receiveBackend
	identity InterfaceIdentity
	now      func() time.Time
	started  time.Time

	queueMu     sync.Mutex
	queue       []Observation
	queueHead   int
	queueCount  int
	queueNotify chan struct{}

	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	state     atomic.Uint32

	resultMu      sync.RWMutex
	terminalError error
	closeError    error

	counters listenerCounters
}

func newListener(backend receiveBackend, config ListenerConfig, now func() time.Time) (*listener, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, fmt.Errorf("%w: nil backend", ErrInvalidConfig)
	}
	if now == nil {
		return nil, fmt.Errorf("%w: nil clock", ErrInvalidConfig)
	}
	identity := backend.Identity()
	if err := identity.validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := &listener{
		backend:     backend,
		identity:    identity,
		queue:       make([]Observation, config.QueueCapacity),
		queueNotify: make(chan struct{}, 1),
		now:         now,
		started:     now(),
		cancel:      cancel,
		done:        make(chan struct{}),
	}
	go result.run(ctx, config.OverflowPolicy)
	return result, nil
}

func (listener *listener) run(ctx context.Context, policy OverflowPolicy) {
	defer close(listener.done)
	defer func() { listener.setCloseError(listener.backend.Close()) }()

	var (
		sequence      uint64
		lastTimestamp time.Duration
	)
	for {
		record, err := listener.backend.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				listener.setTerminalError(ErrClosed)
			} else {
				listener.setTerminalError(fmt.Errorf("canbus: receive backend: %w", err))
			}
			return
		}
		listener.counters.received.Add(1)

		frame, raw, err := decodeSocketCANRecord(record)
		if err != nil {
			listener.counters.rejected.Add(1)
			listener.setTerminalError(fmt.Errorf("canbus: reject received record: %w", err))
			return
		}

		sequence++
		timestamp := listener.now().Sub(listener.started)
		if timestamp < lastTimestamp {
			timestamp = lastTimestamp
		}
		lastTimestamp = timestamp
		observation := Observation{
			frame:     frame,
			identity:  listener.identity,
			sequence:  sequence,
			monotonic: timestamp,
			rawRecord: raw,
		}
		listener.enqueue(observation, policy)
	}
}

func (listener *listener) enqueue(observation Observation, policy OverflowPolicy) {
	listener.queueMu.Lock()
	accepted := false
	if listener.queueCount < len(listener.queue) {
		index := (listener.queueHead + listener.queueCount) % len(listener.queue)
		listener.queue[index] = observation
		listener.queueCount++
		listener.counters.enqueued.Add(1)
		accepted = true
	} else if policy == DropNewest {
		listener.counters.dropped.Add(1)
	} else {
		listener.queue[listener.queueHead] = observation
		listener.queueHead = (listener.queueHead + 1) % len(listener.queue)
		listener.counters.dropped.Add(1)
		listener.counters.enqueued.Add(1)
		accepted = true
	}
	listener.queueMu.Unlock()

	if accepted {
		listener.notifyQueue()
	}
}

func (listener *listener) Receive(ctx context.Context) (Observation, error) {
	if ctx == nil {
		return Observation{}, ErrNilContext
	}
	for {
		if err := ctx.Err(); err != nil {
			return Observation{}, err
		}
		if listener.state.Load() == listenerClosed {
			return Observation{}, ErrClosed
		}
		if observation, ok := listener.dequeue(); ok {
			if listener.state.Load() == listenerClosed {
				return Observation{}, ErrClosed
			}
			listener.counters.delivered.Add(1)
			return observation, nil
		}

		select {
		case <-ctx.Done():
			return Observation{}, ctx.Err()
		case <-listener.queueNotify:
			continue
		case <-listener.done:
			if listener.state.Load() == listenerClosed {
				return Observation{}, ErrClosed
			}
			if observation, ok := listener.dequeue(); ok {
				listener.counters.delivered.Add(1)
				return observation, nil
			}
			return Observation{}, listener.terminal()
		}
	}
}

func (listener *listener) dequeue() (Observation, bool) {
	listener.queueMu.Lock()
	if listener.queueCount == 0 {
		listener.queueMu.Unlock()
		return Observation{}, false
	}
	observation := listener.queue[listener.queueHead]
	listener.queue[listener.queueHead] = Observation{}
	listener.queueHead = (listener.queueHead + 1) % len(listener.queue)
	listener.queueCount--
	remaining := listener.queueCount > 0
	listener.queueMu.Unlock()

	if remaining {
		listener.notifyQueue()
	}
	return observation, true
}

func (listener *listener) notifyQueue() {
	select {
	case listener.queueNotify <- struct{}{}:
	default:
	}
}

func (listener *listener) Stats() ListenerStats {
	return ListenerStats{
		Received:  listener.counters.received.Load(),
		Enqueued:  listener.counters.enqueued.Load(),
		Delivered: listener.counters.delivered.Load(),
		Dropped:   listener.counters.dropped.Load(),
		Rejected:  listener.counters.rejected.Load(),
	}
}

func (listener *listener) Close() error {
	listener.closeOnce.Do(func() {
		listener.state.Store(listenerClosed)
		listener.cancel()
		<-listener.done
	})
	listener.resultMu.RLock()
	defer listener.resultMu.RUnlock()
	return listener.closeError
}

func (listener *listener) setTerminalError(err error) {
	listener.resultMu.Lock()
	defer listener.resultMu.Unlock()
	if listener.terminalError == nil {
		listener.terminalError = err
	}
}

func (listener *listener) setCloseError(err error) {
	listener.resultMu.Lock()
	defer listener.resultMu.Unlock()
	listener.closeError = err
}

func (listener *listener) terminal() error {
	listener.resultMu.RLock()
	defer listener.resultMu.RUnlock()
	if listener.terminalError != nil {
		return listener.terminalError
	}
	return ErrClosed
}

var _ Listener = (*listener)(nil)
