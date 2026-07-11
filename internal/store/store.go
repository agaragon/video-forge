// Package store manages the on-disk locations of uploaded source files and
// encoded outputs. v1 uses the local filesystem; Kickoff.md §3 calls for
// this to stay abstracted behind a small surface so an S3-compatible
// backend can replace it later without touching callers.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileStore allocates unique, path-traversal-safe locations under two
// directories: one for uploaded sources, one for encoded outputs.
type FileStore struct {
	UploadDir string
	OutputDir string
}

// New creates the upload and output directories if needed and returns a
// FileStore rooted at them.
func New(uploadDir, outputDir string) (*FileStore, error) {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{UploadDir: uploadDir, OutputDir: outputDir}, nil
}

var unsafeExt = regexp.MustCompile(`[^a-zA-Z0-9.]`)

// sanitizeExt keeps an upload's original extension (useful for ffprobe/
// ffmpeg format sniffing) while stripping anything that isn't alphanumeric
// or a dot, and bounding its length.
func sanitizeExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	ext = unsafeExt.ReplaceAllString(ext, "")
	if len(ext) > 10 {
		ext = ext[:10]
	}
	return ext
}

// NewUploadPath allocates a unique path for an incoming upload, preserving
// its extension for format sniffing.
func (s *FileStore) NewUploadPath(originalName string) string {
	return filepath.Join(s.UploadDir, newID()+sanitizeExt(originalName))
}

// NewOutputPath allocates a unique path for a job's encoded output in the
// given container format (e.g. "mp4").
func (s *FileStore) NewOutputPath(jobID, containerFormat string) string {
	ext := unsafeExt.ReplaceAllString(strings.ToLower(containerFormat), "")
	return filepath.Join(s.OutputDir, jobID+"."+ext)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
