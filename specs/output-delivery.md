# Output & Delivery

Source: [`Kickoff.md`](../Kickoff.md) §4.4. See [`README.md`](./README.md) for
the EARS legend. Checkbox = implemented in the current codebase.

- [x] **REQ-OUT-1** — When a user requests download of a completed job, the system shall stream the encoded output file. _Implemented: `internal/api.handleDownload` uses `http.ServeContent` to stream the output with a `Content-Disposition: attachment` header._
- [x] **REQ-OUT-2** — The system shall retain each output file for a configurable retention period. _Implemented: `VIDEOFORGE_OUTPUT_RETENTION` (`internal/config`); `Job.complete` stamps `expires_at` at completion time._
- [x] **REQ-OUT-3** — When the retention period for an output elapses, the system shall delete the output file and mark the job as expired. _Implemented: `Manager.retentionSweeper` runs every minute, deletes expired outputs, and transitions matching jobs to `StateExpired`._
- [x] **REQ-OUT-4** — If a user requests an output that has expired or been deleted, then the system shall return a "no longer available" error rather than a generic failure. _Implemented: `handleDownload` returns `410 output_no_longer_available` both when the job state is `expired` and when the output file is missing from disk._
