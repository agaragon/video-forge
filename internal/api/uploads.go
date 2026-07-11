package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/agaragon/video-forge/internal/media"
)

func newUploadID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type uploadResponse struct {
	UploadID string     `json:"upload_id"`
	Filename string     `json:"filename"`
	Info     media.Info `json:"info"`
}

// handleUpload accepts a single multipart file, streams it to disk without
// buffering the whole thing in memory, then probes it. It implements
// REQ-IN-1..6.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// REQ-IN-4: reject uploads over the configured maximum size, reporting the limit.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+1<<20 /* multipart overhead */)

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "expected multipart/form-data with a \"file\" part")
		return
	}

	var part *multipart.Part
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart", "malformed multipart body")
			return
		}
		if p.FormName() == "file" {
			part = p
			break
		}
	}
	if part == nil {
		writeError(w, http.StatusBadRequest, "missing_file", "multipart field \"file\" is required")
		return
	}

	originalName := part.FileName()
	destPath := s.store.NewUploadPath(originalName)
	dest, err := os.Create(destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error", "could not save upload")
		return
	}

	written, copyErr := io.Copy(dest, part)
	closeErr := dest.Close()

	if copyErr != nil {
		os.Remove(destPath)
		var maxErr *http.MaxBytesError
		if errors.As(copyErr, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
				"uploaded file exceeds the maximum allowed size of "+humanBytes(s.cfg.MaxUploadBytes))
			return
		}
		writeError(w, http.StatusBadRequest, "upload_failed", "could not read upload body")
		return
	}
	if closeErr != nil || written == 0 {
		os.Remove(destPath)
		writeError(w, http.StatusBadRequest, "upload_failed", "could not save upload")
		return
	}

	info, err := s.prober.Probe(r.Context(), destPath)
	if err != nil {
		os.Remove(destPath)
		if errors.Is(err, media.ErrUnsupportedFormat) {
			// REQ-IN-5
			writeJSON(w, http.StatusUnsupportedMediaType, unsupportedFormatBody())
			return
		}
		// REQ-IN-6
		writeError(w, http.StatusUnprocessableEntity, "unreadable_file", "uploaded file is unreadable")
		return
	}

	u := &upload{
		ID:           newUploadID(),
		Path:         destPath,
		OriginalName: originalName,
		Info:         info,
		CreatedAt:    time.Now().UTC(),
	}
	s.putUpload(u)

	writeJSON(w, http.StatusCreated, uploadResponse{
		UploadID: u.ID,
		Filename: originalName,
		Info:     info,
	})
}

func unsupportedFormatBody() interface{} {
	type body struct {
		Error struct {
			Code                     string   `json:"code"`
			Message                  string   `json:"message"`
			SupportedInputExtensions []string `json:"supported_input_extensions"`
		} `json:"error"`
	}
	var b body
	b.Error.Code = "unsupported_format"
	b.Error.Message = "uploaded file's format is not supported"
	b.Error.SupportedInputExtensions = media.SupportedInputExtensions
	return b
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
