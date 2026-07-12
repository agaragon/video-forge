package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
}

// generateVideo synthesizes a tiny valid MP4 via ffmpeg's lavfi test source,
// so probe/encode tests don't depend on committed binary fixtures.
func generateVideo(t *testing.T, dir string, duration string) string {
	t.Helper()
	path := filepath.Join(dir, "sample.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration="+duration+":size=64x64:rate=10",
		"-f", "lavfi", "-i", "sine=duration="+duration,
		"-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generating sample video: %v: %s", err, out)
	}
	return path
}

func generateImage(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "sample.png")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-f", "lavfi", "-i", "color=c=red:s=32x32",
		"-frames:v", "1",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generating sample image: %v: %s", err, out)
	}
	return path
}

func TestProbeVideo(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	path := generateVideo(t, dir, "1")

	p := NewProber("ffprobe")
	info, err := p.Probe(context.Background(), path)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if info.Kind != KindVideo {
		t.Errorf("Kind = %q, want video", info.Kind)
	}
	if info.Width != 64 || info.Height != 64 {
		t.Errorf("dimensions = %dx%d, want 64x64", info.Width, info.Height)
	}
	if info.VideoCodec != "h264" {
		t.Errorf("VideoCodec = %q, want h264", info.VideoCodec)
	}
	if info.DurationS < 0.5 {
		t.Errorf("DurationS = %v, want ~1s", info.DurationS)
	}
}

func TestProbeImage(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	path := generateImage(t, dir)

	p := NewProber("ffprobe")
	info, err := p.Probe(context.Background(), path)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if info.Kind != KindImage {
		t.Errorf("Kind = %q, want image", info.Kind)
	}
}

func TestProbeCorruptFile(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.mp4")
	if err := os.WriteFile(path, []byte("this is not a media file"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewProber("ffprobe")
	_, err := p.Probe(context.Background(), path)
	if err == nil {
		t.Fatal("expected an error for a corrupt file")
	}
}
