# Project Kickoff — MediaForge (working title)

A web-based media encoding tool for converting and transcoding **image, audio, and
video** files. Backend written in **Go**; browser-based front end. Requirements are
specified using **EARS** (Easy Approach to Requirements Syntax) so that every
requirement is unambiguous and directly testable.

> Replace "MediaForge" with your final product name. Throughout this document,
> **"the system"** refers to the complete service (front end + Go backend + encoding
> worker).

---

## 1. Vision & Goals

Build a self-hostable web application that lets a user upload a media file, choose an
output format and encoding parameters, and receive a transcoded file — without touching
a command line.

**Primary goals**
- Support the three main media families: image, audio, video.
- Make encoding options discoverable and hard to misconfigure.
- Handle long-running encodes asynchronously with visible progress.
- Ship a clean HTTP API that the web UI (and later, other clients) consume.

**Non-goals (v1)**
- Real-time / live-stream transcoding.
- Collaborative editing or a full media library / DAM.
- Client-side (in-browser WASM) encoding — all encoding happens server-side.

---

## 2. Scope

| In scope (v1) | Out of scope (v1) |
| --- | --- |
| Single-file upload & encode | Batch / multi-file jobs (planned v2) |
| Format + codec + basic parameter selection | Timeline editing, filters, effects |
| Async job queue with progress | Distributed multi-node worker cluster |
| Download of results | Long-term media storage / accounts (optional module) |

---

## 3. Proposed Tech Stack (decisions & assumptions)

- **Language:** Go (backend + encoding worker).
- **Encoding engine:** FFmpeg, invoked as a managed child process. *(Assumption — confirm
  before build; alternative is cgo bindings.)*
- **Transport:** HTTP/JSON API; WebSocket or SSE for progress updates.
- **Job model:** In-process queue for v1; pluggable interface so it can be swapped for
  Redis/NATS later.
- **Front end:** SPA (framework TBD) talking to the Go API.

> Items marked *Assumption* are open decisions — see §8.

---

## 4. EARS Requirements

EARS patterns used below:

- **Ubiquitous** — *The system shall `<response>`* (always active).
- **Event-driven** — *When `<trigger>`, the system shall `<response>`*.
- **State-driven** — *While `<state>`, the system shall `<response>`*.
- **Unwanted behavior** — *If `<trigger>`, then the system shall `<response>`*.
- **Optional** — *Where `<feature>`, the system shall `<response>`*.
- **Complex** — a combination of the above.

Each requirement has a stable ID (`REQ-<area>-<n>`) for traceability to tests.

### 4.1 Media Input & Upload

- **REQ-IN-1** — The system shall accept image, audio, and video files as input.
- **REQ-IN-2** — When a user uploads a file, the system shall validate the container
  format and codec before accepting the job.
- **REQ-IN-3** — When a user uploads a file, the system shall extract and display its
  media metadata (duration, dimensions, codec, bitrate) before encoding.
- **REQ-IN-4** — If an uploaded file exceeds the configured maximum size, then the system
  shall reject the upload and return the size limit in the error message.
- **REQ-IN-5** — If an uploaded file's format is unsupported, then the system shall reject
  the upload and list the supported input formats.
- **REQ-IN-6** — If an uploaded file is corrupt or cannot be probed, then the system shall
  reject the upload and report that the file is unreadable.

### 4.2 Encoding Configuration

- **REQ-CFG-1** — The system shall allow the user to select an output format for the
  uploaded media.
- **REQ-CFG-2** — When a user selects an output format, the system shall present only the
  encoding options valid for that format.
- **REQ-CFG-3** — The system shall allow the user to specify target codec, bitrate,
  resolution (image/video), frame rate (video), and sample rate (audio).
- **REQ-CFG-4** — If the user submits an encoding parameter outside its allowed range,
  then the system shall reject the configuration and state the valid range.
- **REQ-CFG-5** — Where a hardware accelerator (e.g., NVENC, VAAPI, QSV) is available on
  the host, the system shall offer hardware-accelerated encoding as a selectable option.
- **REQ-CFG-6** — Where a preset library is enabled, the system shall let the user apply a
  named preset that fills all encoding parameters at once.

### 4.3 Job Processing

- **REQ-JOB-1** — When a user submits an encoding job, the system shall enqueue the job
  and return a unique job identifier.
- **REQ-JOB-2** — While an encoding job is running, the system shall report progress as a
  percentage of completion.
- **REQ-JOB-3** — While an encoding job is running, the system shall allow the user to
  cancel that job.
- **REQ-JOB-4** — When a user cancels a running job, the system shall stop the encoding
  process and release its resources within a bounded time.
- **REQ-JOB-5** — When an encoding job completes successfully, the system shall make the
  output file available for download.
- **REQ-JOB-6** — While the job queue is at maximum capacity, when a user submits a new
  job, the system shall place the job in a pending state and report its queue position.
- **REQ-JOB-7** — If an encoding job fails, then the system shall record the failure reason
  and notify the user.
- **REQ-JOB-8** — If the encoding engine crashes during a job, then the system shall mark
  the job as failed and free the associated resources.
- **REQ-JOB-9** — If an encoding job exceeds its configured maximum duration, then the
  system shall terminate the job and mark it as timed out.

### 4.4 Output & Delivery

- **REQ-OUT-1** — When a user requests download of a completed job, the system shall stream
  the encoded output file.
- **REQ-OUT-2** — The system shall retain each output file for a configurable retention
  period.
- **REQ-OUT-3** — When the retention period for an output elapses, the system shall delete
  the output file and mark the job as expired.
- **REQ-OUT-4** — If a user requests an output that has expired or been deleted, then the
  system shall return a "no longer available" error rather than a generic failure.

### 4.5 Observability & Operations

- **REQ-OPS-1** — The system shall log every job's lifecycle events (queued, started,
  progress checkpoints, completed, failed).
- **REQ-OPS-2** — The system shall expose a health endpoint reporting service and encoding-
  engine availability.
- **REQ-OPS-3** — While no encoding engine is available, the system shall reject new jobs
  and report that encoding is temporarily unavailable.

### 4.6 Optional Modules (post-v1 candidates)

- **REQ-OPT-1** — Where user accounts are enabled, the system shall associate each job with
  the authenticated user.
- **REQ-OPT-2** — Where batch mode is enabled, the system shall allow multiple files to be
  submitted and tracked as a single job set.
- **REQ-OPT-3** — Where resumable uploads are enabled, if a user loses network connection
  during upload, then the system shall allow the upload to resume from the last received
  chunk.

---

## 5. Non-Functional Requirements

- **NFR-1 (Performance)** — The system shall begin processing a queued job within a
  configurable target latency once a worker is free.
- **NFR-2 (Concurrency)** — The system shall process multiple independent jobs
  concurrently up to a configurable worker limit.
- **NFR-3 (Resource safety)** — The system shall enforce per-job CPU/memory limits so a
  single job cannot exhaust host resources.
- **NFR-4 (Portability)** — The system shall run as a single self-contained deployment
  (Go binary + FFmpeg dependency) on Linux.
- **NFR-5 (Security)** — If an upload attempts to exploit the media engine (malformed
  probe input), then the system shall isolate processing so a compromise cannot affect
  other jobs.

---

## 6. High-Level Architecture

```
Browser (SPA)
    │  HTTP/JSON  +  SSE/WebSocket (progress)
    ▼
Go API server ──► Job Queue ──► Encoding Worker(s) ──► FFmpeg process
    │                                   │
    └──────────► Object/File store ◄────┘   (uploads + outputs)
```

- **API server** — validation, job intake, progress fan-out, download streaming.
- **Job queue** — interface-driven; in-memory for v1.
- **Worker** — pulls jobs, drives FFmpeg, reports progress, writes output.
- **Store** — local filesystem for v1; abstracted for future S3-compatible backends.

---

## 7. Milestones

1. **M0 — Spike:** Go ⇄ FFmpeg process control + progress parsing.
2. **M1 — Core encode path:** upload → configure → encode → download (video only).
3. **M2 — All media families:** image + audio parity with video.
4. **M3 — Robustness:** cancel, timeouts, failures, retention (REQ-JOB-3/7/8/9, REQ-OUT-*).
5. **M4 — Polish & ops:** health, logging, resource limits (§5, §4.5).
6. **M5 — Optional modules:** batch, accounts, resumable uploads (§4.6).

---

## 8. Open Questions / Decisions Needed

- FFmpeg via child process vs. cgo bindings? (affects portability & packaging)
- Which container/codec matrix is officially supported in v1?
- Progress transport: SSE vs. WebSocket?
- Storage backend for v1: local disk only, or S3-compatible from the start?
- Is authentication in scope for v1, or deferred to the optional accounts module?

---

*Requirements in §4 are written in EARS so each maps to one or more test cases. When you
implement, keep the `REQ-*` IDs alongside the tests that verify them for traceability.*
