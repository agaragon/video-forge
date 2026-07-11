# Media Input & Upload

Source: [`Kickoff.md`](../Kickoff.md) §4.1. See [`README.md`](./README.md) for
the EARS legend.

| ID | Requirement | Status |
| --- | --- | --- |
| REQ-IN-1 | The system shall accept image, audio, and video files as input. | Implemented — `internal/media.Probe` classifies uploads into `KindImage`/`KindAudio`/`KindVideo`; `POST /api/v1/uploads` accepts any of them. |
| REQ-IN-2 | When a user uploads a file, the system shall validate the container format and codec before accepting the job. | Implemented — `internal/api.handleUpload` probes the file with ffprobe before it is accepted; a job cannot be created without a successful prior upload+probe. |
| REQ-IN-3 | When a user uploads a file, the system shall extract and display its media metadata (duration, dimensions, codec, bitrate) before encoding. | Implemented — `media.Info` (duration, width/height, codecs, bit rate) is returned in the upload response and shown in the front end's "Configure" step. |
| REQ-IN-4 | If an uploaded file exceeds the configured maximum size, then the system shall reject the upload and return the size limit in the error message. | Implemented — `http.MaxBytesReader` bounded by `VIDEOFORGE_MAX_UPLOAD_BYTES`; `internal/api.handleUpload` returns `413 file_too_large` with the human-readable limit. |
| REQ-IN-5 | If an uploaded file's format is unsupported, then the system shall reject the upload and list the supported input formats. | Implemented — `media.ErrUnsupportedFormat` triggers a `415 unsupported_format` response listing `media.SupportedInputExtensions`. |
| REQ-IN-6 | If an uploaded file is corrupt or cannot be probed, then the system shall reject the upload and report that the file is unreadable. | Implemented — `media.ErrUnprobeable` triggers a `422 unreadable_file` response. |
