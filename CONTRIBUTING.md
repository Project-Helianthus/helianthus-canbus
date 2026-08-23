# Contributing

Open an issue before implementation and keep each change to one issue, branch,
and pull request. Branches use `issue/<id>-<slug>`.

Transport, parser, concurrency, lifecycle, and safety changes require a committed
RED test state before implementation. Tests must use fixtures or injected
receive backends; they must never open a live CAN interface.

Before requesting review, run:

```sh
./scripts/ci_local.sh
```

Pull requests must describe scope, RED evidence, validation commands, queue or
lifecycle effects, and known limitations. P0-P2 review findings are blocking.
After each blocking fix, review the exact current HEAD again. P3-P4 findings are
triaged explicitly but are nonblocking. Merges use squash only.

Contributions must preserve the generic receive-only boundary. Product/profile
semantics, interface or controller configuration, active probing, and any frame
submission path are out of scope.
