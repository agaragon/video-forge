// Package api implements the HTTP/JSON surface described in Kickoff.md §6
// ("API server — validation, job intake, progress fan-out, download
// streaming").
package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/agaragon/video-forge/internal/config"
	"github.com/agaragon/video-forge/internal/job"
	"github.com/agaragon/video-forge/internal/media"
	"github.com/agaragon/video-forge/internal/store"
)

// upload is an accepted-but-not-yet-encoded source file, tracked between
// REQ-IN-* (upload + probe) and REQ-CFG-* (choosing encode parameters).
type upload struct {
	ID           string
	Path         string
	OriginalName string
	Info         media.Info
	CreatedAt    time.Time
}

// Server holds the dependencies backing every HTTP handler.
type Server struct {
	cfg     config.Config
	prober  *media.Prober
	store   *store.FileStore
	manager *job.Manager
	webDir  string

	uploadsMu sync.RWMutex
	uploads   map[string]*upload
}

// NewServer wires up a Server. manager should already have Start called on it.
func NewServer(cfg config.Config, prober *media.Prober, fileStore *store.FileStore, manager *job.Manager, webDir string) *Server {
	return &Server{
		cfg:     cfg,
		prober:  prober,
		store:   fileStore,
		manager: manager,
		webDir:  webDir,
		uploads: make(map[string]*upload),
	}
}

// Routes builds the top-level HTTP handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/v1/formats", s.handleFormats)
	mux.HandleFunc("POST /api/v1/uploads", s.handleUpload)
	mux.HandleFunc("POST /api/v1/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", s.handleCancelJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}/download", s.handleDownload)

	if s.webDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.webDir)))
	}

	return mux
}

func (s *Server) putUpload(u *upload) {
	s.uploadsMu.Lock()
	s.uploads[u.ID] = u
	s.uploadsMu.Unlock()
}

func (s *Server) getUpload(id string) (*upload, bool) {
	s.uploadsMu.RLock()
	defer s.uploadsMu.RUnlock()
	u, ok := s.uploads[id]
	return u, ok
}
