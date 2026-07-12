package media

// SupportedInputExtensions is surfaced to users when an upload is rejected
// for an unsupported format (REQ-IN-5). It documents the v1 container/codec
// matrix rather than gating the upload itself — actual acceptance is
// decided by what ffprobe can identify a recognizable stream in.
var SupportedInputExtensions = []string{
	".mp4", ".mov", ".mkv", ".webm", ".avi", // video
	".mp3", ".wav", ".flac", ".aac", ".ogg", // audio
	".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tiff", // image
}

// OutputFormats enumerates the v1 output containers/codecs offered per
// media Kind, per REQ-CFG-2 ("present only the encoding options valid for
// that format").
var OutputFormats = map[Kind][]string{
	KindVideo: {"mp4", "webm", "mkv"},
	KindAudio: {"mp3", "aac", "flac", "wav", "ogg"},
	KindImage: {"jpg", "png", "webp"},
}
