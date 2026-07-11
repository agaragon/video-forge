# Specifications

This directory splits the requirements in [`../Kickoff.md`](../Kickoff.md) §4–5
into one file per specification set, so each area can evolve (and be reviewed,
linked from PRs, and traced to tests) independently.

Requirements are written in **EARS** (Easy Approach to Requirements Syntax):

- **Ubiquitous** — *The system shall `<response>`* (always active).
- **Event-driven** — *When `<trigger>`, the system shall `<response>`*.
- **State-driven** — *While `<state>`, the system shall `<response>`*.
- **Unwanted behavior** — *If `<trigger>`, then the system shall `<response>`*.
- **Optional** — *Where `<feature>`, the system shall `<response>`*.
- **Complex** — a combination of the above.

Each requirement has a stable ID (`REQ-<area>-<n>` or `NFR-<n>`) for
traceability to tests. When you implement a requirement, keep its ID next to
the test(s) that verify it.

## Specification sets

| File | Area | IDs |
| --- | --- | --- |
| [`input-upload.md`](./input-upload.md) | Media Input & Upload | `REQ-IN-*` |
| [`encoding-configuration.md`](./encoding-configuration.md) | Encoding Configuration | `REQ-CFG-*` |
| [`job-processing.md`](./job-processing.md) | Job Processing | `REQ-JOB-*` |
| [`output-delivery.md`](./output-delivery.md) | Output & Delivery | `REQ-OUT-*` |
| [`observability-operations.md`](./observability-operations.md) | Observability & Operations | `REQ-OPS-*` |
| [`optional-modules.md`](./optional-modules.md) | Optional Modules (post-v1 candidates) | `REQ-OPT-*` |
| [`non-functional-requirements.md`](./non-functional-requirements.md) | Non-Functional Requirements | `NFR-*` |
