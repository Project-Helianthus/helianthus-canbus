package periodic

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
)

var (
	// ErrPeriod reports a period that cannot place frame k at start + k × period.
	ErrPeriod = errors.New("periodic: period is not positive")
	// ErrSuperseded reports a generation that no longer publishes.
	ErrSuperseded = errors.New("periodic: superseded generation publishes nothing")
	// ErrUnknownEntry reports an id this publisher does not hold.
	ErrUnknownEntry = errors.New("periodic: unknown entry")
)

// Snapshot is the last frame accepted for one entry.
// Stale is true after the publisher's generation is superseded.
type Snapshot struct {
	ID    uint64
	Frame canbus.Frame
	Stale bool
}

type entry struct {
	id      uint64
	frame   canbus.Frame
	period  time.Duration
	start   time.Duration
	k       int64
	last    canbus.Frame
	hasLast bool
}

func (e *entry) due() time.Duration {
	return e.start + time.Duration(e.k)*e.period
}

// Publisher sends frames for one interface on the caller's clock.
// Frame k of an entry is due at start + k × period. A late write does not
// move later due times. Supersede stops publication; last accepted frames
// remain readable and are stale.
type Publisher struct {
	name       string
	sender     Sender
	now        func() time.Duration
	mu         sync.Mutex
	entries    map[uint64]*entry
	nextID     uint64
	superseded bool
}

// New returns a publisher for interfaceName. now is the receive clock for
// that interface. sender is the only write path.
func New(interfaceName string, sender Sender, now func() time.Duration) (*Publisher, error) {
	if _, err := canbus.NewInterfaceIdentity(interfaceName, 1); err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, errors.New("periodic: nil sender")
	}
	if now == nil {
		return nil, errors.New("periodic: nil clock")
	}
	return &Publisher{
		name:    interfaceName,
		sender:  sender,
		now:     now,
		entries: map[uint64]*entry{},
	}, nil
}

// Interface returns the interface this publisher serves.
func (p *Publisher) Interface() string { return p.name }

// Add schedules frame at the current clock and every period after that.
// The returned id starts at one.
func (p *Publisher) Add(frame canbus.Frame, period time.Duration) (uint64, error) {
	if period <= 0 {
		return 0, ErrPeriod
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.superseded {
		return 0, ErrSuperseded
	}
	p.nextID++
	id := p.nextID
	p.entries[id] = &entry{id: id, frame: frame, period: period, start: p.now()}
	return id, nil
}

// ReplacePayload changes the payload sent from the next due frame onward.
func (p *Publisher) ReplacePayload(id uint64, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.superseded {
		return ErrSuperseded
	}
	ent, ok := p.entries[id]
	if !ok {
		return ErrUnknownEntry
	}
	frame, err := canbus.NewFrame(ent.frame.ID(), data)
	if err != nil {
		return err
	}
	ent.frame = frame
	return nil
}

// Remove drops an entry. A removed entry sends nothing more.
func (p *Publisher) Remove(id uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.superseded {
		return ErrSuperseded
	}
	if _, ok := p.entries[id]; !ok {
		return ErrUnknownEntry
	}
	delete(p.entries, id)
	return nil
}

// Supersede stops this generation. Later publication calls send nothing.
// Snapshots of frames already accepted are stale.
func (p *Publisher) Supersede() {
	p.mu.Lock()
	p.superseded = true
	p.mu.Unlock()
}

// Command sends one frame through the publisher's sender and does not schedule it.
func (p *Publisher) Command(frame canbus.Frame) (Submission, error) {
	p.mu.Lock()
	superseded := p.superseded
	p.mu.Unlock()
	if superseded {
		return Submission{}, ErrSuperseded
	}
	return p.sendDue(frame)
}

// PublishDue sends every entry whose due time is at or before the current clock.
// Each accepted send advances that entry by one period from its own start.
func (p *Publisher) PublishDue() error {
	for {
		p.mu.Lock()
		if p.superseded {
			p.mu.Unlock()
			return nil
		}
		ent := p.nextDue(p.now())
		if ent == nil {
			p.mu.Unlock()
			return nil
		}
		frame := ent.frame
		id := ent.id
		p.mu.Unlock()

		sub, err := p.sendDue(frame)
		if err != nil {
			return err
		}
		if !sub.Accepted {
			return errors.New("periodic: submission was not accepted")
		}

		p.mu.Lock()
		if current, ok := p.entries[id]; ok && current == ent {
			ent.k++
			ent.last = frame
			ent.hasLast = true
		}
		p.mu.Unlock()
	}
}

// Snapshots returns the last accepted frame of each entry that has sent one,
// ordered by id.
func (p *Publisher) Snapshots() []Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Snapshot, 0, len(p.entries))
	for _, ent := range p.entries {
		if !ent.hasLast {
			continue
		}
		out = append(out, Snapshot{ID: ent.id, Frame: ent.last, Stale: p.superseded})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (p *Publisher) nextDue(now time.Duration) *entry {
	var chosen *entry
	var chosenDue time.Duration
	for _, ent := range p.entries {
		due := ent.due()
		if due > now {
			continue
		}
		if chosen == nil || due < chosenDue || (due == chosenDue && ent.id < chosen.id) {
			chosen = ent
			chosenDue = due
		}
	}
	return chosen
}

func (p *Publisher) sendDue(frame canbus.Frame) (Submission, error) {
	return p.sender.Submit(frame)
}
