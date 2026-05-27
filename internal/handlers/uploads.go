package handlers

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Uploads struct {
	DB  *sql.DB
	Dir string // absolute or cwd-relative directory where image files are written
}

const (
	maxImageSize    = 5 * 1024 * 1024 // 5 MB
	maxFormMemoryMB = 5
)

// Image accepts a multipart/form-data POST with a single "image" file. The
// file is sniffed via http.DetectContentType (looks at the magic bytes —
// extension alone isn't trusted) and saved under Dir with a generated UUID
// filename. The response is { "url": "/uploads/<uuid>.<ext>" }, which is
// then used as the imageUrl on a subsequent POST /api/messages.
func (u *Uploads) Image(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuth(u.DB, w, r); !ok {
		return
	}

	if err := r.ParseMultipartForm(maxFormMemoryMB << 20); err != nil {
		writeError(w, http.StatusBadRequest, "could not parse multipart form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image field is required")
		return
	}
	defer file.Close()

	if header.Size <= 0 || header.Size > maxImageSize {
		writeError(w, http.StatusRequestEntityTooLarge, "image must be 0–5 MB")
		return
	}

	// Sniff first 512 bytes — http.DetectContentType uses magic bytes, not
	// the multipart Content-Type header which clients control.
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "could not read image")
		return
	}
	mime := http.DetectContentType(head[:n])
	ext := extForMime(mime)
	if ext == "" {
		writeError(w, http.StatusBadRequest, "unsupported image type")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		log.Printf("upload seek: %v", err)
		writeError(w, http.StatusInternalServerError, "could not save image")
		return
	}

	if err := os.MkdirAll(u.Dir, 0o755); err != nil {
		log.Printf("upload mkdir: %v", err)
		writeError(w, http.StatusInternalServerError, "could not save image")
		return
	}

	name := uuid.NewString() + ext
	dst := filepath.Join(u.Dir, name)

	out, err := os.Create(dst)
	if err != nil {
		log.Printf("upload create: %v", err)
		writeError(w, http.StatusInternalServerError, "could not save image")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(dst)
		log.Printf("upload write: %v", err)
		writeError(w, http.StatusInternalServerError, "could not save image")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"url": "/uploads/" + name,
	})
}

// extForMime maps the detected MIME to the file extension we'll persist.
// Returning empty means "not an accepted image type".
func extForMime(mime string) string {
	switch strings.TrimSpace(strings.ToLower(mime)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}
