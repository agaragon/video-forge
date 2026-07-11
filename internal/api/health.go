package api

import (
	"context"
	"net/http"
	"os/exec"
	"time"

	"github.com/agaragon/video-forge/internal/media"
)

// healthResponse reports service and encoding-engine availability
// (REQ-OPS-2). Encoding is unavailable when ffmpeg/ffprobe can't be found,
// per REQ-OPS-3.
type healthResponse struct {
	Status           string `json:"status"`
	EncodingEngine   string `json:"encoding_engine"`
	FFmpegAvailable  bool   `json:"ffmpeg_available"`
	FFprobeAvailable bool   `json:"ffprobe_available"`
	QueueLength      int    `json:"queue_length"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	ffmpegOK := checkBinary(ctx, s.cfg.FFmpegPath)
	ffprobeOK := checkBinary(ctx, s.cfg.FFprobePath)

	resp := healthResponse{
		Status:           "ok",
		FFmpegAvailable:  ffmpegOK,
		FFprobeAvailable: ffprobeOK,
		QueueLength:      s.manager.QueueLen(),
	}
	status := http.StatusOK
	if !ffmpegOK || !ffprobeOK {
		resp.Status = "degraded"
		resp.EncodingEngine = "unavailable"
		status = http.StatusServiceUnavailable
	} else {
		resp.EncodingEngine = "available"
	}
	writeJSON(w, status, resp)
}

func checkBinary(ctx context.Context, path string) bool {
	return exec.CommandContext(ctx, path, "-version").Run() == nil
}

type formatsResponse struct {
	SupportedInputExtensions []string                `json:"supported_input_extensions"`
	OutputFormats            map[media.Kind][]string `json:"output_formats"`
}

func (s *Server) handleFormats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, formatsResponse{
		SupportedInputExtensions: media.SupportedInputExtensions,
		OutputFormats:            media.OutputFormats,
	})
}
