# Non-Functional Requirements

Source: [`Kickoff.md`](../Kickoff.md) §5. See [`README.md`](./README.md) for
the EARS legend.

| ID | Requirement | Status |
| --- | --- | --- |
| NFR-1 (Performance) | The system shall begin processing a queued job within a configurable target latency once a worker is free. | Partially implemented — a free worker pulls the next job immediately (`internal/job.Manager.workerLoop`), but there is no configurable target-latency SLO or measurement yet. |
| NFR-2 (Concurrency) | The system shall process multiple independent jobs concurrently up to a configurable worker limit. | Implemented — `VIDEOFORGE_WORKER_COUNT` controls how many `workerLoop` goroutines `Manager.Start` launches. |
| NFR-3 (Resource safety) | The system shall enforce per-job CPU/memory limits so a single job cannot exhaust host resources. | **Not yet implemented.** ffmpeg child processes run without cgroup/rlimit constraints; only wall-clock time is bounded (REQ-JOB-9). |
| NFR-4 (Portability) | The system shall run as a single self-contained deployment (Go binary + FFmpeg dependency) on Linux. | Implemented — `cmd/server` builds to a single static-ish Go binary; the only external dependency is `ffmpeg`/`ffprobe` on `PATH` (see root `README.md`). |
| NFR-5 (Security) | If an upload attempts to exploit the media engine (malformed probe input), then the system shall isolate processing so a compromise cannot affect other jobs. | Partially implemented — each probe/encode runs as a separate OS child process (`os/exec`), which bounds a crash to that process, but there is no sandboxing (seccomp, namespaces, container-per-job) beyond normal process isolation. |
