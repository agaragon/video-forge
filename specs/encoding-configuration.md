# Encoding Configuration

Source: [`Kickoff.md`](../Kickoff.md) §4.2. See [`README.md`](./README.md) for
the EARS legend.

| ID | Requirement | Status |
| --- | --- | --- |
| REQ-CFG-1 | The system shall allow the user to select an output format for the uploaded media. | Implemented — `POST /api/v1/jobs` takes `output_format`; the front end populates the choice from `GET /api/v1/formats`. |
| REQ-CFG-2 | When a user selects an output format, the system shall present only the encoding options valid for that format. | Implemented — `media.OutputFormats` maps each `Kind` to its allowed output formats; `createJobRequest.validate` rejects any `output_format` outside that set for the upload's kind. |
| REQ-CFG-3 | The system shall allow the user to specify target codec, bitrate, resolution (image/video), frame rate (video), and sample rate (audio). | Implemented — `media.Params` carries `VideoCodec`, `AudioCodec`, `BitRate`, `Width`/`Height`, `FrameRate`, `SampleRate`, all wired into the ffmpeg invocation in `media.BuildArgs`. |
| REQ-CFG-4 | If the user submits an encoding parameter outside its allowed range, then the system shall reject the configuration and state the valid range. | Implemented — `internal/api.createJobRequest.validate` bounds width/height/frame rate and checks sample rate against an allow-list, returning `400 invalid_parameters` with the valid range in the message. |
| REQ-CFG-5 | Where a hardware accelerator (e.g., NVENC, VAAPI, QSV) is available on the host, the system shall offer hardware-accelerated encoding as a selectable option. | **Not yet implemented.** Open decision from `Kickoff.md` §8; no hardware-accelerator detection or codec offering exists yet. |
| REQ-CFG-6 | Where a preset library is enabled, the system shall let the user apply a named preset that fills all encoding parameters at once. | **Not yet implemented.** No preset library exists; each request must specify parameters individually. |
