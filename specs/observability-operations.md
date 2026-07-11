# Observability & Operations

Source: [`Kickoff.md`](../Kickoff.md) §4.5. See [`README.md`](./README.md) for
the EARS legend.

| ID | Requirement | Status |
| --- | --- | --- |
| REQ-OPS-1 | The system shall log every job's lifecycle events (queued, started, progress checkpoints, completed, failed). | **Not yet implemented.** The server currently only logs startup/shutdown (`cmd/server/main.go`); `internal/job.Manager` does not emit structured lifecycle log events yet. |
| REQ-OPS-2 | The system shall expose a health endpoint reporting service and encoding-engine availability. | Implemented — `GET /healthz` reports `ffmpeg_available`, `ffprobe_available`, and `queue_length` (`internal/api.handleHealth`). |
| REQ-OPS-3 | While no encoding engine is available, the system shall reject new jobs and report that encoding is temporarily unavailable. | **Not yet implemented.** `/healthz` reflects engine availability, but `POST /api/v1/jobs` does not currently check ffmpeg/ffprobe availability before accepting a job. |
