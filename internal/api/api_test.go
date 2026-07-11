package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/agaragon/video-forge/internal/config"
	"github.com/agaragon/video-forge/internal/job"
	"github.com/agaragon/video-forge/internal/media"
	"github.com/agaragon/video-forge/internal/store"
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

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.UploadDir = dir + "/uploads"
	cfg.OutputDir = dir + "/outputs"
	cfg.MaxUploadBytes = 10 << 20
	cfg.JobTimeout = 20 * time.Second

	fs, err := store.New(cfg.UploadDir, cfg.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	prober := media.NewProber(cfg.FFprobePath)
	encoder := media.NewEncoder(cfg.FFmpegPath)
	manager := job.NewManager(encoder, 1, cfg.JobTimeout, cfg.OutputRetention)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	manager.Start(ctx)

	srv := NewServer(cfg, prober, fs, manager, "")
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return srv, ts
}

func generateVideoBytes(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/sample.mp4"
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=64x64:rate=10",
		"-f", "lavfi", "-i", "sine=duration=1",
		"-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generating sample video: %v: %s", err, out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]interface{}
	json.NewDecoder(res.Body).Decode(&body)
	if _, ok := body["status"]; !ok {
		t.Errorf("expected a status field, got %v", body)
	}
}

func TestUploadRejectsUnsupportedFormat(t *testing.T) {
	requireFFmpeg(t)
	_, ts := newTestServer(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "notes.txt")
	part.Write([]byte("plain text, not media"))
	w.Close()

	res, err := http.Post(ts.URL+"/api/v1/uploads", w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnprocessableEntity && res.StatusCode != http.StatusUnsupportedMediaType {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want 415 or 422; body=%s", res.StatusCode, body)
	}
}

func TestFullJobLifecycle(t *testing.T) {
	requireFFmpeg(t)
	_, ts := newTestServer(t)
	client := ts.Client()

	videoBytes := generateVideoBytes(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "sample.mp4")
	part.Write(videoBytes)
	w.Close()

	res, err := client.Post(ts.URL+"/api/v1/uploads", w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	var uploadResp uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadResp); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", res.StatusCode)
	}
	if uploadResp.Info.Kind != media.KindVideo {
		t.Fatalf("kind = %q, want video", uploadResp.Info.Kind)
	}

	jobReq := createJobRequest{
		UploadID:     uploadResp.UploadID,
		OutputFormat: "webm",
		VideoCodec:   "libvpx",
		SampleRate:   0,
	}
	body, _ := json.Marshal(jobReq)
	res, err = client.Post(ts.URL+"/api/v1/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var jv job.View
	if err := json.NewDecoder(res.Body).Decode(&jv); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("create job status = %d, body=%+v", res.StatusCode, jv)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		res, err := client.Get(ts.URL + "/api/v1/jobs/" + jv.ID)
		if err != nil {
			t.Fatal(err)
		}
		json.NewDecoder(res.Body).Decode(&jv)
		res.Body.Close()
		if jv.State == job.StateCompleted || jv.State == job.StateFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if jv.State != job.StateCompleted {
		t.Fatalf("final state = %q, error=%q", jv.State, jv.Error)
	}

	res, err = client.Get(ts.URL + "/api/v1/jobs/" + jv.ID + "/download")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", res.StatusCode)
	}
	data, _ := io.ReadAll(res.Body)
	if len(data) == 0 {
		t.Error("expected non-empty download body")
	}
}

func TestCreateJobRejectsInvalidUpload(t *testing.T) {
	_, ts := newTestServer(t)
	body, _ := json.Marshal(createJobRequest{UploadID: "does-not-exist", OutputFormat: "mp4"})
	res, err := http.Post(ts.URL+"/api/v1/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}

func TestDownloadNotReady(t *testing.T) {
	_, ts := newTestServer(t)
	res, err := http.Get(ts.URL + "/api/v1/jobs/unknown-id/download")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}
