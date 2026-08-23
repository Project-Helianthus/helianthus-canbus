# Receive Architecture

## Ownership

This module owns generic classic CAN acquisition and evidence transport:

1. `Identifier` and `Frame` validate immutable standard/extended identifiers and
   DLC-bounded payloads.
2. `DecodeSocketCANRecord` converts one fixed native-endian Linux `can_frame`
   record and rejects unsupported or malformed forms.
3. An unexported receive backend isolates OS calls from portable behavior tests.
4. The listener assigns source identity, sequence, and monotonic elapsed time,
   then applies one explicit bounded overflow policy.
5. The Linux backend opens a nonblocking raw CAN socket, disables FD delivery,
   binds it for reception, and checks cancellation with bounded polling.

Profile interpretation and all higher-level semantics are downstream concerns.

## Invariants

- The public `Listener` method set is exactly `Receive`, `Stats`, and `Close`.
- A frame contains no data beyond its validated DLC.
- An observation returns payload and raw record bytes by copy.
- Sequence starts at one and increases for each valid record before queue policy
  is applied. A gap therefore preserves overflow evidence.
- Monotonic elapsed time cannot regress even if an injected clock does.
- Queue capacity is finite and validated before an endpoint is opened.
- Unsupported input terminates visibly; it is never silently treated as data.
- Close cancels acquisition, waits for backend closure, and returns a stable
  close result.

## Platform Boundary

The endpoint implementation is compiled only on Linux. Other platforms compile
the model, decoder, listener tests, and an unsupported-platform opener. Linux
test binaries are cross-compiled during non-Linux local validation and executed
by Linux CI.

Opening and binding a receive endpoint does not establish electrical/controller
listen-only mode. That mode remains an externally managed precondition. The
module does not mutate interface state or inspect physical-layer configuration.
