package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildArgs(t *testing.T) {
	args := BuildArgs("in.mov", "out.mp4", Params{
		VideoCodec: "libx264",
		BitRate:    "2M",
		Width:      1280,
		Height:     720,
		FrameRate:  30,
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{"-i in.mov", "-c:v libx264", "-b:v 2M", "scale=1280:720", "-r 30", "-progress pipe:1", "out.mp4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args %q missing %q", joined, want)
		}
	}
}

func TestEncodeVideoProducesOutputAndProgress(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := generateVideo(t, dir, "1")
	out := filepath.Join(dir, "out.mp4")

	enc := NewEncoder("ffmpeg")
	var checkpoints int
	var sawDone bool
	err := enc.Encode(context.Background(), src, out, Params{
		VideoCodec: "libx264",
		AudioCodec: "aac",
	}, 1.0, func(p Progress) {
		checkpoints++
		if p.Done {
			sawDone = true
		}
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if checkpoints == 0 {
		t.Error("expected at least one progress checkpoint")
	}
	if !sawDone {
		t.Error("expected a final Done checkpoint")
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		t.Errorf("expected non-empty output file, stat err=%v", err)
	}
}

func TestEncodeRespectsCancellation(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()

	// A large, high-frame-count source so ffmpeg (which encodes lavfi
	// sources as fast as the CPU allows, not in real time) can't finish
	// before the cancellation fires.
	src := filepath.Join(dir, "long.mp4")
	genCtx, genCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer genCancel()
	genCmd := exec.CommandContext(genCtx, "ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=60:size=1280x720:rate=30",
		"-c:v", "libx264", "-preset", "ultrafast",
		src,
	)
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("generating long sample video: %v: %s", err, out)
	}

	out := filepath.Join(dir, "out.mp4")
	ctx, cancel := context.WithCancel(context.Background())
	enc := NewEncoder("ffmpeg")

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := enc.Encode(ctx, src, out, Params{VideoCodec: "libx264", BitRate: "500k"}, 60.0, func(Progress) {})
	if err == nil {
		t.Fatal("expected an error after cancellation")
	}
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}
