// Package media wraps ffprobe (inspection) and ffmpeg (encoding) as managed
// child processes, per the "FFmpeg via managed child process" decision in
// Kickoff.md §3.
package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Kind classifies a probed file into one of the three supported media families.
type Kind string

const (
	KindVideo   Kind = "video"
	KindAudio   Kind = "audio"
	KindImage   Kind = "image"
	KindUnknown Kind = "unknown"
)

// ErrUnprobeable is returned when ffprobe cannot make sense of a file at
// all, corresponding to REQ-IN-6 ("corrupt or cannot be probed").
var ErrUnprobeable = errors.New("media: file is corrupt or cannot be probed")

// ErrUnsupportedFormat is returned when ffprobe successfully parses a file
// but it contains no recognizable image/audio/video stream, corresponding
// to REQ-IN-5 ("uploaded file's format is unsupported").
var ErrUnsupportedFormat = errors.New("media: file format is not supported")

// Info is the subset of ffprobe's output that the system surfaces to users,
// per REQ-IN-3 (duration, dimensions, codec, bitrate).
type Info struct {
	Kind       Kind    `json:"kind"`
	Format     string  `json:"format"`     // container format name, e.g. "mov,mp4,m4a"
	DurationS  float64 `json:"duration_s"` // 0 for still images
	BitRate    int64   `json:"bit_rate"`   // bits/sec, 0 if unknown
	VideoCodec string  `json:"video_codec,omitempty"`
	AudioCodec string  `json:"audio_codec,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	SampleRate int     `json:"sample_rate,omitempty"`
	FrameRate  float64 `json:"frame_rate,omitempty"`
}

// ffprobeOutput mirrors the JSON shape produced by `ffprobe -print_format json`.
type ffprobeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		SampleRate   string `json:"sample_rate"`
		RFrameRate   string `json:"r_frame_rate"`
		BitRate      string `json:"bit_rate"`
		DispDuration string `json:"duration"`
	} `json:"streams"`
}

// Prober runs ffprobe against files on disk.
type Prober struct {
	FFprobePath string
}

// NewProber constructs a Prober bound to the given ffprobe binary path.
func NewProber(ffprobePath string) *Prober {
	return &Prober{FFprobePath: ffprobePath}
}

// Probe extracts container/codec/duration/dimension metadata from the file at
// path, implementing REQ-IN-2 (validate before accepting) and REQ-IN-3
// (extract and display metadata).
func (p *Prober) Probe(ctx context.Context, path string) (Info, error) {
	cmd := exec.CommandContext(ctx, p.FFprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Info{}, fmt.Errorf("%w: %v: %s", ErrUnprobeable, err, strings.TrimSpace(stderr.String()))
	}

	var raw ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return Info{}, fmt.Errorf("%w: parsing ffprobe output: %v", ErrUnprobeable, err)
	}
	if raw.Format.FormatName == "" && len(raw.Streams) == 0 {
		return Info{}, ErrUnprobeable
	}

	info := Info{
		Format:    raw.Format.FormatName,
		DurationS: parseFloat(raw.Format.Duration),
		BitRate:   parseInt(raw.Format.BitRate),
	}

	hasVideo, hasAudio := false, false
	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			hasVideo = true
			info.VideoCodec = s.CodecName
			info.Width = s.Width
			info.Height = s.Height
			info.FrameRate = parseFrameRate(s.RFrameRate)
		case "audio":
			hasAudio = true
			info.AudioCodec = s.CodecName
			info.SampleRate = int(parseInt(s.SampleRate))
		}
	}

	switch {
	case hasVideo && isStillImageCodec(info.VideoCodec) && !hasAudio && info.DurationS == 0:
		info.Kind = KindImage
	case hasVideo:
		info.Kind = KindVideo
	case hasAudio:
		info.Kind = KindAudio
	default:
		info.Kind = KindUnknown
	}

	if info.Kind == KindUnknown {
		return info, ErrUnsupportedFormat
	}
	return info, nil
}

// isStillImageCodec reports whether a "video" stream is actually a single
// still frame (mjpeg, png, etc.) as opposed to real video.
func isStillImageCodec(codec string) bool {
	switch codec {
	case "mjpeg", "png", "bmp", "gif", "webp", "tiff", "jpeg2000":
		return true
	default:
		return false
	}
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// parseFrameRate converts ffprobe's "num/den" rational frame rate into a float.
func parseFrameRate(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return parseFloat(s)
	}
	num := parseFloat(parts[0])
	den := parseFloat(parts[1])
	if den == 0 {
		return 0
	}
	return num / den
}

// Duration is a convenience conversion of Info.DurationS to time.Duration.
func (i Info) Duration() time.Duration {
	return time.Duration(i.DurationS * float64(time.Second))
}
