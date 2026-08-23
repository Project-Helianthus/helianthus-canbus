package canbus

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type transportMatrix struct {
	Schema int          `json:"schema"`
	Scope  string       `json:"scope"`
	Cases  []matrixCase `json:"cases"`
}

type matrixCase struct {
	ID                string   `json:"id"`
	Mode              string   `json:"mode"`
	Check             string   `json:"check"`
	Scenario          string   `json:"scenario,omitempty"`
	Value             string   `json:"value,omitempty"`
	Extended          bool     `json:"extended,omitempty"`
	RawID             string   `json:"raw_id,omitempty"`
	DLC               int      `json:"dlc,omitempty"`
	RecordSize        int      `json:"record_size,omitempty"`
	ExpectedError     string   `json:"expected_error,omitempty"`
	ExpectedExtended  bool     `json:"expected_extended,omitempty"`
	Capacity          int      `json:"capacity,omitempty"`
	Count             int      `json:"count,omitempty"`
	Policy            string   `json:"policy,omitempty"`
	ExpectedSequences []uint64 `json:"expected_sequences,omitempty"`
	ExpectedDropped   uint64   `json:"expected_dropped,omitempty"`
	ExpectedEnqueued  uint64   `json:"expected_enqueued,omitempty"`
	Expected          string   `json:"expected"`
}

func TestReceiveOnlyTransportMatrixT01ToT88(t *testing.T) {
	matrix := loadTransportMatrix(t)
	if matrix.Schema != 1 {
		t.Fatalf("schema = %d, want 1", matrix.Schema)
	}
	if matrix.Scope != "classic-can-receive-only" {
		t.Fatalf("scope = %q, want classic-can-receive-only", matrix.Scope)
	}
	if len(matrix.Cases) != 88 {
		t.Fatalf("case count = %d, want exactly 88", len(matrix.Cases))
	}

	seen := make(map[string]struct{}, len(matrix.Cases))
	for index, testCase := range matrix.Cases {
		wantID := fmt.Sprintf("T%02d", index+1)
		if testCase.ID != wantID {
			t.Fatalf("case %d ID = %q, want %q", index, testCase.ID, wantID)
		}
		if _, duplicate := seen[testCase.ID]; duplicate {
			t.Fatalf("duplicate case ID %s", testCase.ID)
		}
		seen[testCase.ID] = struct{}{}
		if testCase.Mode != "receive_only" {
			t.Fatalf("%s mode = %q, want receive_only", testCase.ID, testCase.Mode)
		}
		if testCase.Expected != "pass" {
			t.Fatalf("%s expected = %q, want pass", testCase.ID, testCase.Expected)
		}

		t.Run(testCase.ID+"/"+testCase.Scenario, func(t *testing.T) {
			runMatrixCase(t, testCase)
		})
	}
}

func runMatrixCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	switch testCase.Check {
	case "identifier":
		runMatrixIdentifierCase(t, testCase)
	case "decode":
		runMatrixDecodeCase(t, testCase)
	case "queue":
		runMatrixQueueCase(t, testCase)
	case "config":
		runMatrixConfigCase(t, testCase)
	case "provenance":
		runMatrixProvenanceCase(t, testCase)
	case "lifecycle":
		runMatrixLifecycleCase(t, testCase)
	case "safety":
		runMatrixSafetyCase(t, testCase)
	default:
		t.Fatalf("unknown matrix check %q", testCase.Check)
	}
}

func runMatrixIdentifierCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	value := parseMatrixUint32(t, testCase.Value)
	var err error
	if testCase.Extended {
		_, err = NewExtendedID(value)
	} else {
		_, err = NewStandardID(value)
	}
	assertMatrixError(t, err, testCase.ExpectedError)
}

func runMatrixDecodeCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	recordSize := testCase.RecordSize
	if recordSize == 0 && testCase.Scenario != "empty_record" {
		recordSize = SocketCANRecordSize
	}
	record := make([]byte, recordSize)
	if len(record) >= 4 && testCase.RawID != "" {
		binary.NativeEndian.PutUint32(record[:4], parseMatrixUint32(t, testCase.RawID))
	}
	if len(record) >= 5 {
		record[4] = byte(testCase.DLC)
	}
	if len(record) >= 16 {
		for i := 0; i < MaxClassicDataLength; i++ {
			record[8+i] = byte(i + 1)
		}
	}
	frame, err := DecodeSocketCANRecord(record)
	assertMatrixError(t, err, testCase.ExpectedError)
	if err != nil {
		return
	}
	if frame.DLC() != uint8(testCase.DLC) {
		t.Fatalf("DLC = %d, want %d", frame.DLC(), testCase.DLC)
	}
	if frame.ID().Extended() != testCase.ExpectedExtended {
		t.Fatalf("extended = %t, want %t", frame.ID().Extended(), testCase.ExpectedExtended)
	}
	if testCase.Scenario == "trailing_bytes_hidden" {
		if frame.DLC() != 2 || len(frame.Data()) != 2 || frame.Bytes()[2] != 0 {
			t.Fatalf("bytes beyond DLC are visible: DLC=%d data=%x bytes=%x", frame.DLC(), frame.Data(), frame.Bytes())
		}
	}
}

func runMatrixQueueCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	backend := newFakeReceiveBackend(t)
	for i := 0; i < testCase.Count; i++ {
		backend.reads <- fakeBackendRead{record: socketCANFixture(uint32(0x300+i), 1, byte(i))}
	}
	policy := DropNewest
	if testCase.Policy == "drop_oldest" {
		policy = DropOldest
	}
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: testCase.Capacity, OverflowPolicy: policy}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	waitForStats(t, listener, func(stats ListenerStats) bool { return stats.Received == uint64(testCase.Count) })
	stats := listener.Stats()
	if stats.Dropped != testCase.ExpectedDropped || stats.Enqueued != testCase.ExpectedEnqueued {
		t.Fatalf("stats = %+v, want dropped=%d enqueued=%d", stats, testCase.ExpectedDropped, testCase.ExpectedEnqueued)
	}
	for _, sequence := range testCase.ExpectedSequences {
		if got := receiveObservation(t, listener).Sequence(); got != sequence {
			t.Fatalf("sequence = %d, want %d", got, sequence)
		}
	}
}

func runMatrixConfigCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	policy := DropNewest
	if testCase.Policy == "invalid" {
		policy = OverflowPolicy(255)
	}
	called := false
	_, err := listenSocketCAN("test-can0", ListenerConfig{QueueCapacity: testCase.Capacity, OverflowPolicy: policy}, func(string) (receiveBackend, error) {
		called = true
		return newFakeReceiveBackend(t), nil
	}, time.Now)
	assertMatrixError(t, err, testCase.ExpectedError)
	if called {
		t.Fatal("backend opener ran for invalid bounded-queue configuration")
	}
}

func runMatrixProvenanceCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	knownScenarios := map[string]bool{
		"sequence_starts_at_one": true, "sequence_increments": true,
		"interface_name": true, "interface_index": true,
		"elapsed_timestamp": true, "timestamp_clamped": true,
		"raw_record_exact": true, "raw_record_copy": true,
		"frame_data_copy": true, "dlc_bounds_evidence": true,
		"extended_flag_evidence": true, "received_stats": true,
	}
	if !knownScenarios[testCase.Scenario] {
		t.Fatalf("unknown provenance scenario %q", testCase.Scenario)
	}

	backend := newFakeReceiveBackend(t)
	firstRecord := socketCANFixture(0x321, 2, 0xaa, 0xbb)
	secondRecord := socketCANFixture(testExtendedFlag|0x12345, 1, 0xcc)
	backend.reads <- fakeBackendRead{record: firstRecord}
	backend.reads <- fakeBackendRead{record: secondRecord}
	base := time.Unix(1_700_000_000, 0)
	clock := &scriptedClock{values: []time.Time{base, base.Add(10 * time.Millisecond), base.Add(5 * time.Millisecond)}}
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 2, OverflowPolicy: DropNewest}, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	waitForStats(t, listener, func(stats ListenerStats) bool { return stats.Received == 2 })
	first := receiveObservation(t, listener)
	second := receiveObservation(t, listener)

	switch testCase.Scenario {
	case "sequence_starts_at_one":
		if first.Sequence() != 1 {
			t.Fatalf("sequence = %d", first.Sequence())
		}
	case "sequence_increments":
		if second.Sequence() != first.Sequence()+1 {
			t.Fatalf("sequences = %d, %d", first.Sequence(), second.Sequence())
		}
	case "interface_name":
		if first.Interface().Name() != "test-can0" {
			t.Fatalf("name = %q", first.Interface().Name())
		}
	case "interface_index":
		if first.Interface().Index() != 17 {
			t.Fatalf("index = %d", first.Interface().Index())
		}
	case "elapsed_timestamp":
		if first.MonotonicTimestamp() != 10*time.Millisecond {
			t.Fatalf("timestamp = %s", first.MonotonicTimestamp())
		}
	case "timestamp_clamped":
		if second.MonotonicTimestamp() < first.MonotonicTimestamp() {
			t.Fatalf("timestamps regressed")
		}
	case "raw_record_exact":
		if got := first.RawRecord(); !bytesEqual(got[:], firstRecord) {
			t.Fatalf("raw = %x", got)
		}
	case "raw_record_copy":
		copyOfRaw := first.RawRecord()
		copyOfRaw[8] = 0
		if first.RawRecord()[8] != 0xaa {
			t.Fatal("raw record aliases caller")
		}
	case "frame_data_copy":
		copyOfData := first.Frame().Data()
		copyOfData[0] = 0
		if first.Frame().Data()[0] != 0xaa {
			t.Fatal("frame data aliases caller")
		}
	case "dlc_bounds_evidence":
		if first.Frame().DLC() != 2 || len(first.Frame().Data()) != 2 {
			t.Fatalf("frame = %#v", first.Frame())
		}
	case "extended_flag_evidence":
		if !second.Frame().ID().Extended() {
			t.Fatal("extended evidence lost")
		}
	case "received_stats":
		if listener.Stats().Received != 2 {
			t.Fatalf("stats = %+v", listener.Stats())
		}
	}
}

func runMatrixLifecycleCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	switch testCase.Scenario {
	case "canceled_context":
		listener, backend := matrixEmptyListener(t)
		defer listener.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := listener.Receive(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
		_ = backend
	case "deadline_context":
		listener, _ := matrixEmptyListener(t)
		defer listener.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		_, err := listener.Receive(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v", err)
		}
	case "blocked_receive_close":
		listener, _ := matrixEmptyListener(t)
		result := make(chan error, 1)
		go func() { _, err := listener.Receive(context.Background()); result <- err }()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-result:
			if !errors.Is(err, ErrClosed) {
				t.Fatalf("error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("receive remained blocked")
		}
	case "close_idempotent":
		listener, _ := matrixEmptyListener(t)
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	case "backend_close_once":
		listener, backend := matrixEmptyListener(t)
		_ = listener.Close()
		_ = listener.Close()
		if backend.closeCalls.Load() != 1 {
			t.Fatalf("close calls = %d", backend.closeCalls.Load())
		}
	case "backend_error":
		backend := newFakeReceiveBackend(t)
		failure := errors.New("matrix backend failure")
		backend.reads <- fakeBackendRead{err: failure}
		listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		_, err = listener.Receive(context.Background())
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	case "unsupported_record_error":
		backend := newFakeReceiveBackend(t)
		backend.reads <- fakeBackendRead{record: socketCANFixture(testRTRFlag|1, 0)}
		listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		_, err = listener.Receive(context.Background())
		if !errors.Is(err, ErrUnsupportedRTR) {
			t.Fatalf("error = %v", err)
		}
	case "receive_after_close":
		listener, _ := matrixEmptyListener(t)
		_ = listener.Close()
		_, err := listener.Receive(context.Background())
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("error = %v", err)
		}
	case "nil_context":
		listener, _ := matrixEmptyListener(t)
		defer listener.Close()
		_, err := listener.Receive(nil)
		if !errors.Is(err, ErrNilContext) {
			t.Fatalf("error = %v", err)
		}
	case "close_error_stable":
		listener, backend := matrixEmptyListener(t)
		backend.closeErr = errors.New("matrix close failure")
		first := listener.Close()
		second := listener.Close()
		if !errors.Is(first, backend.closeErr) || !errors.Is(second, backend.closeErr) {
			t.Fatalf("errors = %v, %v", first, second)
		}
	case "opener_error":
		failure := errors.New("matrix opener failure")
		_, err := listenSocketCAN("test-can0", ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, func(string) (receiveBackend, error) { return nil, failure }, time.Now)
		if !errors.Is(err, failure) {
			t.Fatalf("error = %v", err)
		}
	case "config_before_opener":
		called := false
		_, err := listenSocketCAN("test-can0", ListenerConfig{}, func(string) (receiveBackend, error) { called = true; return nil, nil }, time.Now)
		if !errors.Is(err, ErrInvalidConfig) || called {
			t.Fatalf("error = %v called=%t", err, called)
		}
	default:
		t.Fatalf("unknown lifecycle scenario %q", testCase.Scenario)
	}
}

func runMatrixSafetyCase(t *testing.T, testCase matrixCase) {
	t.Helper()
	switch testCase.Scenario {
	case "tx_surface_denied":
		typeOfListener := reflect.TypeOf((*Listener)(nil)).Elem()
		methods := make([]string, typeOfListener.NumMethod())
		for i := range methods {
			methods[i] = typeOfListener.Method(i).Name
		}
		sort.Strings(methods)
		if !reflect.DeepEqual(methods, []string{"Close", "Receive", "Stats"}) {
			t.Fatalf("methods = %v", methods)
		}
	case "tx_syscalls_denied", "netlink_mutation_denied", "public_transport_handle_denied":
		if err := inspectProductSource("."); err != nil {
			t.Fatal(err)
		}
	case "live_io_not_invoked":
		if err := inspectTestSourceForEndpointCalls(); err != nil {
			t.Fatal(err)
		}
	case "linux_build_tag":
		contents, err := os.ReadFile("socketcan_linux.go")
		if err != nil || !strings.Contains(string(contents), "//go:build linux") {
			t.Fatalf("linux build tag: %v", err)
		}
	case "portable_build_tag":
		contents, err := os.ReadFile("socketcan_unsupported.go")
		if err != nil || !strings.Contains(string(contents), "//go:build !linux") {
			t.Fatalf("portable build tag: %v", err)
		}
	case "classic_record_fixed_size":
		if SocketCANRecordSize != 16 {
			t.Fatalf("record size = %d", SocketCANRecordSize)
		}
	case "fd_explicitly_unsupported":
		_, err := DecodeSocketCANRecord(make([]byte, SocketCANFDRecordSize))
		if !errors.Is(err, ErrUnsupportedFD) {
			t.Fatalf("error = %v", err)
		}
	case "queue_has_finite_upper_bound":
		if MaxQueueCapacity <= 0 || MaxQueueCapacity > 1<<20 {
			t.Fatalf("maximum queue capacity = %d", MaxQueueCapacity)
		}
	default:
		t.Fatalf("unknown safety scenario %q", testCase.Scenario)
	}
}

func matrixEmptyListener(t *testing.T) (Listener, *fakeReceiveBackend) {
	t.Helper()
	backend := newFakeReceiveBackend(t)
	listener, err := newListener(backend, ListenerConfig{QueueCapacity: 1, OverflowPolicy: DropNewest}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return listener, backend
}

func inspectTestSourceForEndpointCalls() error {
	for _, path := range goFilesFromRoot(".", true) {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		var found string
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if identifier.Name == "ListenSocketCAN" || identifier.Name == "platformOpenSocketCAN" {
				found = identifier.Name
				return false
			}
			return true
		})
		if found != "" {
			return fmt.Errorf("%s invokes live endpoint entry point %s", path, found)
		}
	}
	return nil
}

func loadTransportMatrix(t *testing.T) transportMatrix {
	t.Helper()
	contents, err := os.ReadFile("testdata/transport_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix transportMatrix
	if err := json.Unmarshal(contents, &matrix); err != nil {
		t.Fatal(err)
	}
	return matrix
}

func parseMatrixUint32(t *testing.T, value string) uint32 {
	t.Helper()
	parsed, err := strconv.ParseUint(value, 0, 32)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return uint32(parsed)
}

func assertMatrixError(t *testing.T, got error, expected string) {
	t.Helper()
	if expected == "" {
		if got != nil {
			t.Fatalf("unexpected error: %v", got)
		}
		return
	}
	want := map[string]error{
		"invalid_identifier":      ErrInvalidIdentifier,
		"invalid_dlc":             ErrInvalidDLC,
		"invalid_record_length":   ErrInvalidRecordLength,
		"unsupported_fd":          ErrUnsupportedFD,
		"unsupported_rtr":         ErrUnsupportedRTR,
		"unsupported_error_frame": ErrUnsupportedErrorFrame,
		"invalid_config":          ErrInvalidConfig,
	}[expected]
	if want == nil {
		t.Fatalf("unknown expected error %q", expected)
	}
	if !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
}
