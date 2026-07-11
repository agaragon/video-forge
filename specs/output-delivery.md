# Output & Delivery

Source: [`Kickoff.md`](../Kickoff.md) §4.4. See [`README.md`](./README.md) for
the EARS legend.

| ID | Requirement | Status |
| --- | --- | --- |
| REQ-OUT-1 | When a user requests download of a completed job, the system shall stream the encoded output file. | Implemented — `internal/api.handleDownload` uses `http.ServeContent` to stream the output with a `Content-Disposition: attachment` header. |
| REQ-OUT-2 | The system shall retain each output file for a configurable retention period. | Implemented — `VIDEOFORGE_OUTPUT_RETENTION` (`internal/config`); `Job.complete` stamps `expires_at` at completion time. |
| REQ-OUT-3 | When the retention period for an output elapses, the system shall delete the output file and mark the job as expired. | Implemented — `Manager.retentionSweeper` runs every minute, deletes expired outputs, and transitions matching jobs to `StateExpired`. |
| REQ-OUT-4 | If a user requests an output that has expired or been deleted, then the system shall return a "no longer available" error rather than a generic failure. | Implemented — `handleDownload` returns `410 output_no_longer_available` both when the job state is `expired` and when the output file is missing from disk. |
