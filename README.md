# video-forge

A self-hostable web app for transcoding image, audio, and video files. Go
backend + a small vanilla-JS front end, driving FFmpeg as a managed child
process. Built from [`Kickoff.md`](./Kickoff.md), which is the source of
truth for scope and requirements (`REQ-*` IDs below refer to it).

Status: **M1 — core encode path** (upload → configure → encode → download),
across all three media families, plus cancellation, timeouts, and output
retention (M2/M3 from the kickoff's milestone list).

## Requirements

- Go 1.24+
- `ffmpeg` and `ffprobe` on `PATH` (or point `VIDEOFORGE_FFMPEG_PATH` /
  `VIDEOFORGE_FFPROBE_PATH` at them)

## Run

```sh
make run          # serves on :8080
```

Then open http://localhost:8080 — upload a file, pick an output format, and
watch it encode.

## Test

```sh
make test         # runs unit + integration tests (some skip without ffmpeg)
```

Tests in `internal/media`, `internal/job`, and `internal/api` generate their
own throwaway sample media via `ffmpeg -f lavfi`, so no binary fixtures are
committed to the repo.

## Configuration

All config is via `VIDEOFORGE_*` environment variables (see
`internal/config/config.go` for the full list and defaults):

| Variable | Default | Meaning |
| --- | --- | --- |
| `VIDEOFORGE_ADDR` | `:8080` | HTTP listen address |
| `VIDEOFORGE_UPLOAD_DIR` | `scratch/uploads` | Where uploads land before encoding |
| `VIDEOFORGE_OUTPUT_DIR` | `scratch/outputs` | Where encoded outputs are held |
| `VIDEOFORGE_MAX_UPLOAD_BYTES` | 2 GiB | REQ-IN-4 upload size ceiling |
| `VIDEOFORGE_WORKER_COUNT` | 2 | Concurrent encode workers (NFR-2) |
| `VIDEOFORGE_JOB_TIMEOUT` | 30m | REQ-JOB-9 max job duration |
| `VIDEOFORGE_OUTPUT_RETENTION` | 24h | REQ-OUT-2/3 output retention window |

## Architecture

```
Browser (vanilla JS SPA)
    │  HTTP/JSON  +  SSE (progress)
    ▼
internal/api  ──►  internal/job (queue + worker pool)  ──►  internal/media (ffmpeg/ffprobe)
    │                                                              │
    └──────────────────►  internal/store (local filesystem)  ◄────┘
```

- **`internal/media`** — wraps `ffprobe` (metadata extraction, REQ-IN-2/3)
  and `ffmpeg` (encoding with `-progress pipe:1` for REQ-JOB-2).
- **`internal/job`** — in-memory FIFO queue behind a small `Queue` interface
  (swappable for Redis/NATS per Kickoff.md §3) and a worker pool implementing
  the REQ-JOB-* lifecycle, including cancellation (REQ-JOB-3/4), timeouts
  (REQ-JOB-9), and retention-based expiry (REQ-OUT-3).
- **`internal/store`** — allocates unique on-disk paths for uploads/outputs.
- **`internal/api`** — HTTP handlers: upload, job intake, SSE progress
  events, cancel, download.
- **`web/static`** — the front end: upload → configure → progress → download.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Service + encoding-engine health (REQ-OPS-2/3) |
| `GET` | `/api/v1/formats` | Supported input extensions and per-kind output formats (REQ-CFG-2) |
| `POST` | `/api/v1/uploads` | Upload + probe a file (`multipart/form-data`, field `file`) |
| `POST` | `/api/v1/jobs` | Submit an encode job for a prior upload (REQ-JOB-1) |
| `GET` | `/api/v1/jobs/{id}` | Job status snapshot |
| `GET` | `/api/v1/jobs/{id}/events` | SSE stream of progress checkpoints (REQ-JOB-2) |
| `DELETE` | `/api/v1/jobs/{id}` | Cancel a pending/running job (REQ-JOB-3/4) |
| `GET` | `/api/v1/jobs/{id}/download` | Stream the completed output (REQ-OUT-1) |

## What's not implemented yet

Per Kickoff.md's non-goals and open questions: hardware-accelerated encoding
(REQ-CFG-5), named presets (REQ-CFG-6), and the optional modules — accounts,
batch jobs, resumable uploads (§4.6) — are out of scope for this milestone.
The job queue is in-memory and not persisted across restarts.
