# Encoding Configuration

Source: [`Kickoff.md`](../Kickoff.md) §4.2. See [`README.md`](./README.md) for
the EARS legend. Checkbox = implemented in the current codebase.

- [x] **REQ-CFG-1** — The system shall allow the user to select an output format for the uploaded media. _Implemented: `POST /api/v1/jobs` takes `output_format`; the front end populates the choice from `GET /api/v1/formats`._
- [x] **REQ-CFG-2** — When a user selects an output format, the system shall present only the encoding options valid for that format. _Implemented: `media.OutputFormats` maps each `Kind` to its allowed output formats; `createJobRequest.validate` rejects any `output_format` outside that set for the upload's kind._
- [x] **REQ-CFG-3** — The system shall allow the user to specify target codec, bitrate, resolution (image/video), frame rate (video), and sample rate (audio). _Implemented: `media.Params` carries `VideoCodec`, `AudioCodec`, `BitRate`, `Width`/`Height`, `FrameRate`, `SampleRate`, all wired into the ffmpeg invocation in `media.BuildArgs`._
- [x] **REQ-CFG-4** — If the user submits an encoding parameter outside its allowed range, then the system shall reject the configuration and state the valid range. _Implemented: `internal/api.createJobRequest.validate` bounds width/height/frame rate and checks sample rate against an allow-list, returning `400 invalid_parameters` with the valid range in the message._
- [ ] **REQ-CFG-5** — Where a hardware accelerator (e.g., NVENC, VAAPI, QSV) is available on the host, the system shall offer hardware-accelerated encoding as a selectable option. _Not implemented: open decision from `Kickoff.md` §8; no hardware-accelerator detection or codec offering exists yet._
- [ ] **REQ-CFG-6** — Where a preset library is enabled, the system shall let the user apply a named preset that fills all encoding parameters at once. _Not implemented: no preset library exists; each request must specify parameters individually._
