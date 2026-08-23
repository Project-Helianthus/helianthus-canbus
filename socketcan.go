package canbus

import (
	"errors"
	"fmt"
	"time"
)

type backendOpener func(string) (receiveBackend, error)

// ListenSocketCAN opens a classic CAN receive endpoint on Linux.
//
// The controller's electrical listen-only state is an external precondition.
// This library neither configures nor verifies controller or interface state.
func ListenSocketCAN(interfaceName string, config ListenerConfig) (Listener, error) {
	return listenSocketCAN(interfaceName, config, platformOpenSocketCAN, time.Now)
}

func listenSocketCAN(interfaceName string, config ListenerConfig, opener backendOpener, now func() time.Time) (Listener, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if opener == nil {
		return nil, fmt.Errorf("%w: nil backend opener", ErrInvalidConfig)
	}
	backend, err := opener(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("canbus: open SocketCAN receive endpoint: %w", err)
	}
	if backend == nil {
		return nil, fmt.Errorf("%w: backend opener returned nil", ErrInvalidConfig)
	}
	result, err := newListener(backend, config, now)
	if err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	return result, nil
}
