package periodic

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/Project-Helianthus/helianthus-canbus"
)

type fakeClock struct{ now time.Duration }

func (c *fakeClock) Now() time.Duration { return c.now }

func mustFrame(t *testing.T, extended bool, value uint32, data []byte) canbus.Frame {
	t.Helper()
	var (
		id  canbus.Identifier
		err error
	)
	if extended {
		id, err = canbus.NewExtendedID(value)
	} else {
		id, err = canbus.NewStandardID(value)
	}
	if err != nil {
		t.Fatal(err)
	}
	frame, err := canbus.NewFrame(id, data)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func mustPublisher(t *testing.T, name string, sim *Simulated, clock *fakeClock) *Publisher {
	t.Helper()
	publisher, err := New(name, sim, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	if publisher.Interface() != name {
		t.Fatalf("interface = %s", publisher.Interface())
	}
	return publisher
}

func TestGridStaysOnStart(t *testing.T) {
	t.Parallel()
	period := 10 * time.Millisecond
	start := time.Second
	clock := &fakeClock{now: start}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frame := mustFrame(t, false, 0x42, []byte{1, 2, 3, 4})
	if _, err := publisher.Add(frame, 0); !errors.Is(err, ErrPeriod) {
		t.Fatalf("period error = %v", err)
	}
	if _, err := publisher.Add(frame, period); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		at := start + time.Duration(i)*period
		clock.now = at
		if err := publisher.PublishDue(); err != nil {
			t.Fatal(err)
		}
		got := sim.Frames()
		if len(got) != i+1 {
			t.Fatalf("frames = %d, want %d", len(got), i+1)
		}
		if got[i].Monotonic != at {
			t.Fatalf("frame %d time = %v, want %v", i, got[i].Monotonic, at)
		}
		assertFrame(t, got[i].Frame, frame)
	}
	if snaps := publisher.Snapshots(); len(snaps) != 1 || snaps[0].Stale {
		t.Fatalf("snapshots = %+v", snaps)
	}
}

func TestDelayDoesNotMoveLaterGrid(t *testing.T) {
	t.Parallel()
	start := time.Second
	period := 10 * time.Millisecond
	delay := 4 * time.Millisecond
	clock := &fakeClock{now: start}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frame := mustFrame(t, false, 0x11, []byte{9})
	if _, err := publisher.Add(frame, period); err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	clock.now = start + period
	sim.before = func() { clock.now += delay }
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	clock.now = start + 2*period
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	got := sim.Frames()
	if len(got) != 3 {
		t.Fatalf("frames = %d", len(got))
	}
	want := []time.Duration{start, start + period + delay, start + 2*period}
	for i, at := range want {
		if got[i].Monotonic != at {
			t.Fatalf("frame %d time = %v, want %v", i, got[i].Monotonic, at)
		}
	}
}

func TestPeriodsKeepIndependentGrids(t *testing.T) {
	t.Parallel()
	start := 100 * time.Millisecond
	periodP := 10 * time.Millisecond
	periodQ := 15 * time.Millisecond
	clock := &fakeClock{now: start}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frameP := mustFrame(t, false, 0x100, []byte{1})
	frameQ := mustFrame(t, false, 0x200, []byte{2})
	if _, err := publisher.Add(frameP, periodP); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.Add(frameQ, periodQ); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Duration{start, start + 10*time.Millisecond, start + 15*time.Millisecond, start + 20*time.Millisecond, start + 30*time.Millisecond} {
		clock.now = at
		if err := publisher.PublishDue(); err != nil {
			t.Fatal(err)
		}
	}
	got := sim.Frames()
	wantData := []byte{1, 2, 1, 2, 1, 1, 2}
	wantAt := []time.Duration{
		start, start,
		start + 10*time.Millisecond,
		start + 15*time.Millisecond,
		start + 20*time.Millisecond,
		start + 30*time.Millisecond, start + 30*time.Millisecond,
	}
	if len(got) != len(wantData) {
		t.Fatalf("frames = %d", len(got))
	}
	for i := range wantData {
		if got[i].Monotonic != wantAt[i] || len(got[i].Frame.Data()) != 1 || got[i].Frame.Data()[0] != wantData[i] {
			t.Fatalf("frame %d = %v data %v", i, got[i].Monotonic, got[i].Frame.Data())
		}
	}
}

func TestReplacePayloadShowsOnNextSend(t *testing.T) {
	t.Parallel()
	start := time.Second
	period := 5 * time.Millisecond
	clock := &fakeClock{now: start}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frame := mustFrame(t, false, 0x21, []byte{1, 1})
	id, err := publisher.Add(frame, period)
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	if err := publisher.ReplacePayload(id, []byte{4, 5, 6}); err != nil {
		t.Fatal(err)
	}
	clock.now = start + period
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	got := sim.Frames()
	if len(got) != 2 || !bytes.Equal(got[0].Frame.Data(), []byte{1, 1}) || !bytes.Equal(got[1].Frame.Data(), []byte{4, 5, 6}) {
		t.Fatalf("payloads = %v %v", got[0].Frame.Data(), got[1].Frame.Data())
	}
	if got[1].Frame.DLC() != 3 || got[1].Frame.ID() != frame.ID() {
		t.Fatalf("replaced frame = dlc %d id %v", got[1].Frame.DLC(), got[1].Frame.ID())
	}
}

func TestRemovedEntrySendsNothingMore(t *testing.T) {
	t.Parallel()
	start := time.Second
	period := 5 * time.Millisecond
	clock := &fakeClock{now: start}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	keep := mustFrame(t, false, 0x31, []byte{7})
	drop := mustFrame(t, false, 0x32, []byte{8})
	keepID, err := publisher.Add(keep, period)
	if err != nil {
		t.Fatal(err)
	}
	dropID, err := publisher.Add(drop, period)
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Remove(dropID); err != nil {
		t.Fatal(err)
	}
	clock.now = start + period
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	got := sim.Frames()
	if len(got) != 3 {
		t.Fatalf("frames = %d", len(got))
	}
	if got[2].Frame.ID() != keep.ID() || got[2].Frame.Data()[0] != 7 {
		t.Fatalf("last frame = id %v data %v", got[2].Frame.ID(), got[2].Frame.Data())
	}
	if err := publisher.ReplacePayload(keepID, []byte{1}); err != nil {
		t.Fatal(err)
	}
}

func TestCommandSendsOneFrame(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: 20 * time.Millisecond}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frame := mustFrame(t, true, 0x1ABCDE, []byte{7, 8, 9})
	sub, err := publisher.Command(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	got := sim.Frames()
	if len(got) != 1 {
		t.Fatalf("frames = %d", len(got))
	}
	assertFrame(t, got[0].Frame, frame)
	if !sub.Accepted || sub.Monotonic != clock.now || got[0].Monotonic != sub.Monotonic {
		t.Fatalf("submission = %+v frame time %v", sub, got[0].Monotonic)
	}
	if !frame.ID().Extended() || frame.DLC() != 3 {
		t.Fatalf("command id extended=%v dlc=%d", frame.ID().Extended(), frame.DLC())
	}
}

func TestReceiptOrdersLocalCompletion(t *testing.T) {
	t.Parallel()
	clock := &stepClock{}
	sim := NewSimulated(clock.Now)
	frame := mustFrame(t, false, 0x7, []byte{1})
	before := clock.Now()
	sub, observed, err := func() (Submission, time.Duration, error) {
		sub, err := sim.Submit(frame)
		return sub, clock.Now(), err
	}()
	if err != nil {
		t.Fatal(err)
	}
	if before >= sub.Monotonic || sub.Monotonic >= observed {
		t.Fatalf("before %v receipt %v observed %v", before, sub.Monotonic, observed)
	}
	if !sub.Accepted || len(sim.Frames()) != 1 || sim.Frames()[0].Monotonic != sub.Monotonic {
		t.Fatalf("submission = %+v", sub)
	}
}

func TestFailedWriteHasNoAcceptedReceipt(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: time.Second}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frame := mustFrame(t, false, 0x8, []byte{1})
	sim.fail = errors.New("synthetic write failure")
	sub, err := publisher.Command(frame)
	if err == nil || sub.Accepted || sub.Monotonic != 0 || len(sim.Frames()) != 0 {
		t.Fatalf("submission = %+v err = %v frames = %d", sub, err, len(sim.Frames()))
	}
	if _, err := publisher.Add(frame, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	sim.fail = errors.New("synthetic write failure")
	if err := publisher.PublishDue(); err == nil || len(sim.Frames()) != 0 || len(publisher.Snapshots()) != 0 {
		t.Fatal(err)
	}
}

func TestSupersededGenerationPublishesNothing(t *testing.T) {
	t.Parallel()
	start := time.Second
	period := time.Millisecond
	clock := &fakeClock{now: start}
	sim := NewSimulated(clock.Now)
	publisher := mustPublisher(t, "synthetic0", sim, clock)
	frame := mustFrame(t, false, 0x9, []byte{3})
	if _, err := publisher.Add(frame, period); err != nil {
		t.Fatal(err)
	}
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	if snaps := publisher.Snapshots(); len(snaps) != 1 || snaps[0].Stale {
		t.Fatalf("snapshots = %+v", snaps)
	}
	publisher.Supersede()
	clock.now = start + period
	if err := publisher.PublishDue(); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.Command(frame); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("command error = %v", err)
	}
	if _, err := publisher.Add(frame, period); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("add error = %v", err)
	}
	if len(sim.Frames()) != 1 {
		t.Fatalf("frames = %d", len(sim.Frames()))
	}
	snaps := publisher.Snapshots()
	if len(snaps) != 1 || !snaps[0].Stale || snaps[0].Frame.ID() != frame.ID() {
		t.Fatalf("snapshots = %+v", snaps)
	}
}

func TestPublishersOnTwoInterfacesStayApart(t *testing.T) {
	t.Parallel()
	clockA := &fakeClock{now: time.Second}
	clockB := &fakeClock{now: 2 * time.Second}
	simA := NewSimulated(clockA.Now)
	simB := NewSimulated(clockB.Now)
	pubA := mustPublisher(t, "synthetic0", simA, clockA)
	pubB := mustPublisher(t, "synthetic1", simB, clockB)
	frameA := mustFrame(t, false, 0xA, []byte{1})
	frameB := mustFrame(t, false, 0xB, []byte{2, 2})
	if _, err := pubA.Add(frameA, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := pubB.Add(frameB, 3*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := pubA.PublishDue(); err != nil {
		t.Fatal(err)
	}
	if err := pubB.PublishDue(); err != nil {
		t.Fatal(err)
	}
	gotA := simA.Frames()
	gotB := simB.Frames()
	if len(gotA) != 1 || len(gotB) != 1 {
		t.Fatalf("frames %d %d", len(gotA), len(gotB))
	}
	assertFrame(t, gotA[0].Frame, frameA)
	assertFrame(t, gotB[0].Frame, frameB)
	if gotA[0].Monotonic != clockA.now || gotB[0].Monotonic != clockB.now {
		t.Fatalf("times %v %v", gotA[0].Monotonic, gotB[0].Monotonic)
	}
}

func TestClassicRecordLayout(t *testing.T) {
	t.Parallel()
	standard := mustFrame(t, false, 0x123, []byte{1, 2, 3, 4})
	raw := recordBytes(standard)
	if id := binary.NativeEndian.Uint32(raw[:4]); id != 0x123 || raw[4] != 4 || raw[5] != 0 || raw[6] != 0 || raw[7] != 0 {
		t.Fatalf("standard record = %x", raw)
	}
	if !bytes.Equal(raw[8:12], []byte{1, 2, 3, 4}) {
		t.Fatalf("payload = %x", raw[8:12])
	}
	extended := mustFrame(t, true, 0x1ABCDE, []byte{9})
	raw = recordBytes(extended)
	if id := binary.NativeEndian.Uint32(raw[:4]); id != (0x1ABCDE|extendedFlag) || raw[4] != 1 || raw[8] != 9 {
		t.Fatalf("extended record = %x", raw)
	}
}

type stepClock struct{ n time.Duration }

func (c *stepClock) Now() time.Duration {
	c.n += time.Millisecond
	return c.n
}

func assertFrame(t *testing.T, got, want canbus.Frame) {
	t.Helper()
	if got.ID() != want.ID() || got.DLC() != want.DLC() || !bytes.Equal(got.Data(), want.Data()) {
		t.Fatalf("frame id=%v dlc=%d data=%v, want id=%v dlc=%d data=%v", got.ID(), got.DLC(), got.Data(), want.ID(), want.DLC(), want.Data())
	}
}
