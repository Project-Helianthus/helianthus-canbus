// Command replay feeds published receive vectors to the moved openers.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/Project-Helianthus/helianthus-canbus"
	"github.com/Project-Helianthus/helianthus-canbus/receiver"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: replay input.json expected.json")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var casePartition = map[string]string{
	"linux-bms-receive":            "linux",
	"linux-bms-identity-mismatch":  "linux",
	"linux-bms-missing-interface":  "linux",
	"linux-project-reopen-clock":   "linux",
	"nonlinux-project-unsupported": "nonlinux",
	"nonlinux-bms-unsupported":     "nonlinux",
}

type inputDoc struct {
	Format    string `json:"format"`
	Case      string `json:"case"`
	Partition string `json:"partition"`
	Operands  struct {
		Names []string `json:"names"`
	} `json:"operands"`
}

type expectedDoc struct {
	Format    string            `json:"format"`
	Case      string            `json:"case"`
	Partition string            `json:"partition"`
	Calls     []json.RawMessage `json:"calls"`
	Result    struct {
		Attempts []struct {
			Name          string `json:"name"`
			IsConfig      *bool  `json:"is_config_error"`
			IsUnsupported *bool  `json:"is_unsupported_error"`
			Outcome       struct {
				Error            string `json:"error"`
				ReceiverReturned bool   `json:"receiver_returned"`
				SocketActions    int    `json:"socket_actions"`
			} `json:"outcome"`
		} `json:"attempts"`
	} `json:"result"`
}

func run(inputPath, expectedPath string) error {
	input, err := readJSON[inputDoc](inputPath)
	if err != nil {
		return err
	}
	expected, err := readJSON[expectedDoc](expectedPath)
	if err != nil {
		return err
	}
	if input.Case != expected.Case || input.Partition != expected.Partition {
		return errors.New("input and expected case differ")
	}
	partition, ok := casePartition[input.Case]
	if !ok || partition != input.Partition {
		return fmt.Errorf("unknown case %s", input.Case)
	}
	if input.Format != "helianthus-oracle-qv05-input/1" || expected.Format != "helianthus-oracle-qv05-expected/1" {
		return errors.New("unexpected vector format")
	}
	if input.Partition == "linux" {
		if runtime.GOOS != "linux" {
			return fmt.Errorf("platform mismatch: case partition linux, host %s", runtime.GOOS)
		}
		return driveLinux(input, expected)
	}
	if runtime.GOOS == "linux" {
		return fmt.Errorf("platform mismatch: case partition nonlinux, host %s", runtime.GOOS)
	}
	return compareNonLinux(input, expected)
}

func readJSON[T any](path string) (T, error) {
	var doc T
	data, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	return doc, nil
}

func compareNonLinux(input inputDoc, expected expectedDoc) error {
	if len(expected.Calls) != 0 {
		return errors.New("expected no operating-system calls")
	}
	if len(input.Operands.Names) != len(expected.Result.Attempts) {
		return fmt.Errorf("name count %d, attempts %d", len(input.Operands.Names), len(expected.Result.Attempts))
	}
	for i, name := range input.Operands.Names {
		attempt := expected.Result.Attempts[i]
		if attempt.Name != name {
			return fmt.Errorf("attempt %d name differs", i)
		}
		if attempt.Outcome.Error == "" || attempt.Outcome.ReceiverReturned || attempt.Outcome.SocketActions != 0 {
			return fmt.Errorf("attempt %d recorded a receiver or a socket action", i)
		}
		switch input.Case {
		case "nonlinux-project-unsupported":
			opened, err := receiver.Open(name)
			if opened != nil {
				return fmt.Errorf("open returned a receiver for %q", name)
			}
			if err == nil || err.Error() != unsupportedText() || !errors.Is(err, canbus.ErrUnsupportedPlatform) {
				return fmt.Errorf("Open(%q) error = %v, want %s", name, err, unsupportedText())
			}
		case "nonlinux-bms-unsupported":
			opened, err := receiver.OpenClassic(name)
			if opened != nil {
				return fmt.Errorf("open classic returned a receiver for %q", name)
			}
			invalidName := !receiver.ValidInterfaceName(name)
			want := unsupportedText()
			wantConfig, wantUnsupported := false, true
			if invalidName {
				want = invalidText()
				wantConfig, wantUnsupported = true, false
			}
			if err == nil || err.Error() != want {
				return fmt.Errorf("OpenClassic(%q) error = %v, want %s", name, err, want)
			}
			if invalidName && !errors.Is(err, canbus.ErrInvalidInterface) {
				return fmt.Errorf("OpenClassic(%q) error = %v", name, err)
			}
			if !invalidName && !errors.Is(err, canbus.ErrUnsupportedPlatform) {
				return fmt.Errorf("OpenClassic(%q) error = %v", name, err)
			}
			if attempt.IsConfig == nil || *attempt.IsConfig != wantConfig || attempt.IsUnsupported == nil || *attempt.IsUnsupported != wantUnsupported {
				return fmt.Errorf("attempt %d classification differs", i)
			}
		default:
			return fmt.Errorf("case %s is not a non-linux opener", input.Case)
		}
	}
	return nil
}

func unsupportedText() string {
	return fmt.Errorf("%w: %s", canbus.ErrUnsupportedPlatform, runtime.GOOS).Error()
}

func invalidText() string {
	return canbus.ErrInvalidInterface.Error()
}
