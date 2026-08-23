# Repository Rules

## Scope

- Keep production code and public APIs generic CAN transport only.
- Keep product, profile, register, property, and domain semantics outside this
  repository.
- This milestone is classic CAN and receive-only. Do not add frame submission,
  active probing, interface configuration, netlink mutation, or controller
  configuration.
- Treat externally configured controller listen-only mode as a precondition,
  never as a state this library claims to configure or verify.
- Do not open a live CAN interface from tests. Use the receive-backend seam and
  byte fixtures.

## Workflow

- Use one issue, one `issue/<id>-<slug>` branch, and one PR at a time.
- Use RED-first tests for transport, concurrency, lifecycle, and safety behavior.
- Keep the T01..T88 matrix exact, machine-readable, receive-only, and green.
- Run `./scripts/ci_local.sh` before review.
- Resolve P0-P2 findings, then obtain a fresh exact-HEAD no-blocker review.
- Merge only with squash after applicable gates pass. Do not merge as part of
  implementation work unless the operator explicitly requests that boundary.
- Use initiator/responder terminology.

Do not add plan hashes, authority tokens, attestations, or executable workflow
state to this repository.
