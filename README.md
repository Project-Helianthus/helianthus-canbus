# helianthus-canbus

`helianthus-canbus` is the Project Helianthus generic receive-only classic CAN
foundation. This repository owns the reusable frame model, Linux SocketCAN
acquisition, bounded evidence queue, provenance, and lifecycle behavior. Product,
profile, register, and domain semantics belong outside this module.

## Scope

- Standard 11-bit and extended 29-bit identifiers.
- Classic CAN payloads with DLC `0..8`.
- Native Linux SocketCAN records (`struct can_frame`).
- Immutable frame and raw-record copies with interface identity, listener-local
  sequence, and monotonic elapsed time.
- Explicit bounded-queue policies: `DropNewest` and `DropOldest`.
- Deterministic context cancellation and idempotent close.
- Explicit rejection of CAN FD, RTR, error frames, malformed records, and
  out-of-range identifiers or DLC values.

The public transport surface contains only `Receive`, `Stats`, and `Close`.
There is no frame-submission API, active probing, interface mutation, or
controller configuration.

## Safety Boundary

`ListenSocketCAN` opens and binds a receive endpoint. Electrical/controller
listen-only operation is an **external precondition** that must be configured by
the system owner before this library opens the endpoint. The module does not
configure or verify controller mode, interface state, link timing, termination,
or physical-layer behavior. Its receive-only API is not proof that the external
controller is electrically silent.

No test opens a platform endpoint. Listener behavior is tested through an
injected receive backend and deterministic byte fixtures.

## Usage

```go
listener, err := canbus.ListenSocketCAN("can0", canbus.ListenerConfig{
    QueueCapacity:  256,
    OverflowPolicy: canbus.DropOldest,
})
if err != nil {
    return err
}
defer listener.Close()

observation, err := listener.Receive(ctx)
```

Callers should monitor `ListenerStats.Dropped` and sequence gaps as explicit
overflow evidence.

## Validation

```sh
./scripts/ci_local.sh
```

The local gate runs terminology and forbidden-surface checks, formatting, vet,
build, race tests, Linux compilation, and the exact receive-only T01..T88
matrix. On Linux, the same suite compiles and executes the Linux implementation;
on other systems, Linux test binaries are cross-compiled only.

## License

Copyright and licensing terms are in [LICENSE](LICENSE). The module is released
under AGPL-3.0.
