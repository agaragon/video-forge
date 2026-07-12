package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Params describes the requested output for one encode job, per REQ-CFG-1..3.
type Params struct {
	OutputFormat string  `json:"output_format"` // container/muxer, e.g. "mp4", "webp", "mp3"
	VideoCodec   string  `json:"video_codec,omitempty"`
	AudioCodec   string  `json:"audio_codec,omitempty"`
	BitRate      string  `json:"bit_rate,omitempty"` // e.g. "2M", "128k"; empty means "use codec default"
	Width        int     `json:"width,omitempty"`    // 0 means "keep source"
	Height       int     `json:"height,omitempty"`   // 0 means "keep source"
	FrameRate    float64 `json:"frame_rate,omitempty"`
	SampleRate   int     `json:"sample_rate,omitempty"`
}

// Progress is one reported checkpoint of an in-flight encode (REQ-JOB-2).
type Progress struct {
	Percent    float64 `json:"percent"`
	OutTimeS   float64 `json:"out_time_s"`
	Speed      string  `json:"speed,omitempty"`
	FrameCount int64   `json:"frame_count,omitempty"`
	Done       bool    `json:"done"`
}

// Encoder runs ffmpeg to transcode a source file into Params.OutputFormat.
type Encoder struct {
	FFmpegPath string
}

// NewEncoder constructs an Encoder bound to the given ffmpeg binary path.
func NewEncoder(ffmpegPath string) *Encoder {
	return &Encoder{FFmpegPath: ffmpegPath}
}

// BuildArgs turns Params into ffmpeg CLI flags. Exported for testability.
func BuildArgs(inputPath, outputPath string, p Params) []string {
	args := []string{"-y", "-i", inputPath}

	if p.VideoCodec != "" {
		args = append(args, "-c:v", p.VideoCodec)
	}
	if p.AudioCodec != "" {
		args = append(args, "-c:a", p.AudioCodec)
	}
	if p.BitRate != "" {
		args = append(args, "-b:v", p.BitRate)
	}
	if p.Width > 0 && p.Height > 0 {
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", p.Width, p.Height))
	}
	if p.FrameRate > 0 {
		args = append(args, "-r", strconv.FormatFloat(p.FrameRate, 'f', -1, 64))
	}
	if p.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(p.SampleRate))
	}

	// Machine-readable progress on stdout, one "key=value" block per checkpoint.
	args = append(args, "-progress", "pipe:1", "-nostats")
	args = append(args, outputPath)
	return args
}

// Encode runs ffmpeg to produce outputPath from inputPath, streaming progress
// checkpoints to onProgress as they arrive (REQ-JOB-2). totalDuration is the
// probed source duration, used to turn "out_time" into a percentage; pass 0
// for still images, in which case Percent jumps straight to 100 on completion.
//
// The context governs cancellation (REQ-JOB-3/4) and timeout (REQ-JOB-9): if
// ctx is cancelled or times out, the ffmpeg process is killed and Encode
// returns ctx.Err().
func (e *Encoder) Encode(ctx context.Context, inputPath, outputPath string, p Params, totalDurationS float64, onProgress func(Progress)) error {
	args := BuildArgs(inputPath, outputPath, p)
	cmd := exec.CommandContext(ctx, e.FFmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("media: stdout pipe: %w", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("media: starting ffmpeg: %w", err)
	}

	scanErr := make(chan error, 1)
	go func() {
		scanErr <- scanProgress(stdout, totalDurationS, onProgress)
	}()

	waitErr := cmd.Wait()
	<-scanErr

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if waitErr != nil {
		return fmt.Errorf("media: ffmpeg failed: %w: %s", waitErr, strings.TrimSpace(lastLines(stderr.String(), 10)))
	}

	onProgress(Progress{Percent: 100, Done: true})
	return nil
}

// scanProgress reads ffmpeg's "-progress pipe:1" key=value stream and emits a
// Progress checkpoint each time a "progress=" line closes out a block.
func scanProgress(r io.Reader, totalDurationS float64, onProgress func(Progress)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var cur Progress
	var outTimeUs int64
	var frame int64
	var speed string

	for sc.Scan() {
		line := sc.Text()
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "out_time_us":
			outTimeUs, _ = strconv.ParseInt(val, 10, 64)
		case "frame":
			frame, _ = strconv.ParseInt(val, 10, 64)
		case "speed":
			speed = val
		case "progress":
			cur.OutTimeS = float64(outTimeUs) / 1_000_000
			cur.FrameCount = frame
			cur.Speed = speed
			if totalDurationS > 0 {
				pct := (cur.OutTimeS / totalDurationS) * 100
				if pct > 99.9 {
					pct = 99.9 // reserve 100 for the final "done" checkpoint
				}
				if pct < 0 {
					pct = 0
				}
				cur.Percent = pct
			}
			cur.Done = val == "end"
			onProgress(cur)
		}
	}
	return sc.Err()
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
