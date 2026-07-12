# Observability & Operations

Source: [`Kickoff.md`](../Kickoff.md) §4.5. See [`README.md`](./README.md) for
the EARS legend. Checkbox = implemented in the current codebase.

- [ ] **REQ-OPS-1** — The system shall log every job's lifecycle events (queued, started, progress checkpoints, completed, failed). _Not implemented: the server currently only logs startup/shutdown (`cmd/server/main.go`); `internal/job.Manager` does not emit structured lifecycle log events yet._
- [x] **REQ-OPS-2** — The system shall expose a health endpoint reporting service and encoding-engine availability. _Implemented: `GET /healthz` reports `ffmpeg_available`, `ffprobe_available`, and `queue_length` (`internal/api.handleHealth`)._
- [ ] **REQ-OPS-3** — While no encoding engine is available, the system shall reject new jobs and report that encoding is temporarily unavailable. _Not implemented: `/healthz` reflects engine availability, but `POST /api/v1/jobs` does not currently check ffmpeg/ffprobe availability before accepting a job._
