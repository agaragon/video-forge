// Package config holds runtime configuration for the video-forge server.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all tunables for the API server, job queue, and worker pool.
type Config struct {
	// Addr is the address the HTTP server listens on, e.g. ":8080".
	Addr string

	// UploadDir is where incoming media files are stored before encoding.
	UploadDir string
	// OutputDir is where encoded outputs are stored until they expire.
	OutputDir string

	// MaxUploadBytes is the maximum accepted size of an uploaded file (REQ-IN-4).
	MaxUploadBytes int64

	// WorkerCount bounds how many encoding jobs run concurrently (NFR-2).
	WorkerCount int
	// QueueCapacity bounds how many jobs may be pending at once (REQ-JOB-6).
	QueueCapacity int

	// JobTimeout is the maximum wall-clock duration a single job may run (REQ-JOB-9).
	JobTimeout time.Duration
	// OutputRetention is how long a completed output stays downloadable (REQ-OUT-2/3).
	OutputRetention time.Duration

	// FFmpegPath and FFprobePath allow overriding the binaries used to encode/probe media.
	FFmpegPath  string
	FFprobePath string
}

// Default returns the built-in configuration, then applies VIDEOFORGE_* env overrides.
func Default() Config {
	c := Config{
		Addr:            ":8080",
		UploadDir:       "scratch/uploads",
		OutputDir:       "scratch/outputs",
		MaxUploadBytes:  2 << 30, // 2 GiB
		WorkerCount:     2,
		QueueCapacity:   100,
		JobTimeout:      30 * time.Minute,
		OutputRetention: 24 * time.Hour,
		FFmpegPath:      "ffmpeg",
		FFprobePath:     "ffprobe",
	}

	if v := os.Getenv("VIDEOFORGE_ADDR"); v != "" {
		c.Addr = v
	}
	if v := os.Getenv("VIDEOFORGE_UPLOAD_DIR"); v != "" {
		c.UploadDir = v
	}
	if v := os.Getenv("VIDEOFORGE_OUTPUT_DIR"); v != "" {
		c.OutputDir = v
	}
	if v := os.Getenv("VIDEOFORGE_MAX_UPLOAD_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.MaxUploadBytes = n
		}
	}
	if v := os.Getenv("VIDEOFORGE_WORKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.WorkerCount = n
		}
	}
	if v := os.Getenv("VIDEOFORGE_QUEUE_CAPACITY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.QueueCapacity = n
		}
	}
	if v := os.Getenv("VIDEOFORGE_JOB_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.JobTimeout = d
		}
	}
	if v := os.Getenv("VIDEOFORGE_OUTPUT_RETENTION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.OutputRetention = d
		}
	}
	if v := os.Getenv("VIDEOFORGE_FFMPEG_PATH"); v != "" {
		c.FFmpegPath = v
	}
	if v := os.Getenv("VIDEOFORGE_FFPROBE_PATH"); v != "" {
		c.FFprobePath = v
	}

	return c
}
