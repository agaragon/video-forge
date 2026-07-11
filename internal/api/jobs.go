package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/agaragon/video-forge/internal/job"
	"github.com/agaragon/video-forge/internal/media"
)

// createJobRequest is the body of POST /api/v1/jobs (REQ-CFG-1..3).
type createJobRequest struct {
	UploadID     string  `json:"upload_id"`
	OutputFormat string  `json:"output_format"`
	VideoCodec   string  `json:"video_codec,omitempty"`
	AudioCodec   string  `json:"audio_codec,omitempty"`
	BitRate      string  `json:"bit_rate,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
	FrameRate    float64 `json:"frame_rate,omitempty"`
	SampleRate   int     `json:"sample_rate,omitempty"`
}

const (
	minDimension = 1
	maxDimension = 7680 // 8K width/height ceiling
	maxFrameRate = 240
)

var validSampleRates = map[int]bool{
	0: true, 8000: true, 11025: true, 16000: true, 22050: true,
	32000: true, 44100: true, 48000: true, 96000: true,
}

// validate checks req against the v1 parameter ranges, implementing
// REQ-CFG-2 (format-appropriate options) and REQ-CFG-4 (reject out-of-range
// values, stating the valid range).
func (req createJobRequest) validate(kind media.Kind) (media.Params, string, bool) {
	allowed := media.OutputFormats[kind]
	ok := false
	for _, f := range allowed {
		if f == req.OutputFormat {
			ok = true
			break
		}
	}
	if !ok {
		return media.Params{}, fmt.Sprintf("output_format must be one of %v for %s input", allowed, kind), false
	}

	if req.Width != 0 && (req.Width < minDimension || req.Width > maxDimension) {
		return media.Params{}, fmt.Sprintf("width must be between %d and %d", minDimension, maxDimension), false
	}
	if req.Height != 0 && (req.Height < minDimension || req.Height > maxDimension) {
		return media.Params{}, fmt.Sprintf("height must be between %d and %d", minDimension, maxDimension), false
	}
	if req.FrameRate != 0 && (req.FrameRate < 0 || req.FrameRate > maxFrameRate) {
		return media.Params{}, fmt.Sprintf("frame_rate must be between 0 and %d", maxFrameRate), false
	}
	if !validSampleRates[req.SampleRate] {
		return media.Params{}, "sample_rate must be one of 8000, 11025, 16000, 22050, 32000, 44100, 48000, 96000", false
	}

	return media.Params{
		OutputFormat: req.OutputFormat,
		VideoCodec:   req.VideoCodec,
		AudioCodec:   req.AudioCodec,
		BitRate:      req.BitRate,
		Width:        req.Width,
		Height:       req.Height,
		FrameRate:    req.FrameRate,
		SampleRate:   req.SampleRate,
	}, "", true
}

// handleCreateJob implements REQ-JOB-1: enqueue a job and return its ID.
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	u, ok := s.getUpload(req.UploadID)
	if !ok {
		writeError(w, http.StatusNotFound, "upload_not_found", "no upload found for upload_id")
		return
	}

	params, reason, ok := req.validate(u.Info.Kind)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_parameters", reason)
		return
	}

	id := job.NewID()
	outputPath := s.store.NewOutputPath(id, params.OutputFormat)
	j := s.manager.Submit(id, u.Info.Kind, u.Path, outputPath, u.Info, params)

	writeJSON(w, http.StatusAccepted, j.Snapshot())
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job_not_found", "no job found for id")
		return
	}
	writeJSON(w, http.StatusOK, j.Snapshot())
}

// handleCancelJob implements REQ-JOB-3/4: cancel a running or pending job.
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.manager.Cancel(id)
	switch {
	case errors.Is(err, job.ErrNotFound):
		writeError(w, http.StatusNotFound, "job_not_found", "no job found for id")
	case errors.Is(err, job.ErrNotCancelable):
		writeError(w, http.StatusConflict, "not_cancelable", "job has already finished")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "cancel_failed", err.Error())
	default:
		j, _ := s.manager.Get(id)
		writeJSON(w, http.StatusOK, j.Snapshot())
	}
}

// handleJobEvents streams progress checkpoints over SSE while a job runs
// (REQ-JOB-2), ending once the job reaches a terminal state or the client
// disconnects.
func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job_not_found", "no job found for id")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "server does not support streaming")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	send := func() bool {
		snap := j.Snapshot()
		data, _ := json.Marshal(snap)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return isTerminal(snap.State)
	}

	if send() {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if send() {
				return
			}
		}
	}
}

func isTerminal(s job.State) bool {
	switch s {
	case job.StateCompleted, job.StateFailed, job.StateCancelled, job.StateTimedOut, job.StateExpired:
		return true
	default:
		return false
	}
}

// handleDownload implements REQ-OUT-1 (stream completed output) and
// REQ-OUT-4 (a distinct "no longer available" error for expired/deleted
// output rather than a generic failure).
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job_not_found", "no job found for id")
		return
	}

	switch j.State() {
	case job.StateExpired:
		writeError(w, http.StatusGone, "output_no_longer_available", "this output is no longer available")
		return
	case job.StateCompleted:
		// fall through to stream the file
	default:
		writeError(w, http.StatusConflict, "not_ready", "job has not completed yet")
		return
	}

	f, err := os.Open(j.OutputPath)
	if err != nil {
		writeError(w, http.StatusGone, "output_no_longer_available", "this output is no longer available")
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "could not read output file")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+"."+j.Params.OutputFormat))
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), f)
}
