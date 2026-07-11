# Optional Modules (post-v1 candidates)

Source: [`Kickoff.md`](../Kickoff.md) §4.6. See [`README.md`](./README.md) for
the EARS legend. These are explicitly out of scope for v1 (see
`Kickoff.md` §2 "Out of scope") and tracked here for when M5 begins.

| ID | Requirement | Status |
| --- | --- | --- |
| REQ-OPT-1 | Where user accounts are enabled, the system shall associate each job with the authenticated user. | Not implemented — no authentication/authorization exists; deferred to M5. |
| REQ-OPT-2 | Where batch mode is enabled, the system shall allow multiple files to be submitted and tracked as a single job set. | Not implemented — `POST /api/v1/uploads` and `POST /api/v1/jobs` are single-file only; deferred to M5. |
| REQ-OPT-3 | Where resumable uploads are enabled, if a user loses network connection during upload, then the system shall allow the upload to resume from the last received chunk. | Not implemented — uploads are single-shot `multipart/form-data`; deferred to M5. |
