package common

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/moto-nrw/project-phoenix/internal/randstr"
	"github.com/moto-nrw/project-phoenix/internal/storage"
)

// AllowedImageTypes maps MIME types detected by http.DetectContentType to allowed image formats.
var AllowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
}

// AllowedPDFTypes maps MIME types detected by http.DetectContentType to allowed PDF documents.
var AllowedPDFTypes = map[string]bool{
	"application/pdf": true,
}

// UploadedFile holds the result of parsing and validating an image upload.
type UploadedFile struct {
	File        io.ReadSeekCloser
	Filename    string
	ContentType string
}

// ParseImage parses a multipart form upload, validates the content type via magic bytes,
// and returns the file ready for saving. The caller must close UploadedFile.File.
func ParseImage(w http.ResponseWriter, r *http.Request, fieldName string, maxSize int64) (*UploadedFile, error) {
	return ParseImageWithLimits(w, r, fieldName, maxSize, maxSize)
}

// ParseImageWithLimits enforces an advertised file-size cap separately from
// the multipart body cap. Body cap needs headroom for boundaries and headers.
func ParseImageWithLimits(w http.ResponseWriter, r *http.Request, fieldName string, maxFileSize, maxBodySize int64) (*UploadedFile, error) {
	return parseUploadWithLimits(w, r, fieldName, maxFileSize, maxBodySize, detectImageContentType)
}

// ParsePDFWithLimits enforces an advertised file-size cap separately from
// the multipart body cap. Body cap needs headroom for boundaries and headers.
func ParsePDFWithLimits(w http.ResponseWriter, r *http.Request, fieldName string, maxFileSize, maxBodySize int64) (*UploadedFile, error) {
	return parseUploadWithLimits(w, r, fieldName, maxFileSize, maxBodySize, detectPDFContentType)
}

func parseUploadWithLimits(
	w http.ResponseWriter,
	r *http.Request,
	fieldName string,
	maxFileSize int64,
	maxBodySize int64,
	detect func(io.ReadSeeker) (string, error),
) (*UploadedFile, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		return nil, errors.New("file too large")
	}

	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, errors.New("no file uploaded")
	}
	if header.Size > maxFileSize {
		_ = file.Close()
		return nil, errors.New("file too large")
	}

	contentType, err := detect(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	return &UploadedFile{
		File:        file,
		Filename:    header.Filename,
		ContentType: contentType,
	}, nil
}

// detectImageContentType reads the first 512 bytes to detect the MIME type via magic bytes
// and validates it against AllowedImageTypes.
func detectImageContentType(file io.ReadSeeker) (string, error) {
	return detectAllowedContentType(file, AllowedImageTypes, "invalid file type. Only JPEG, PNG, and WebP images are allowed")
}

// detectPDFContentType reads the first 512 bytes to detect a PDF via magic bytes.
func detectPDFContentType(file io.ReadSeeker) (string, error) {
	return detectAllowedContentType(file, AllowedPDFTypes, "invalid file type. Only PDF documents are allowed")
}

func detectAllowedContentType(file io.ReadSeeker, allowed map[string]bool, invalidMessage string) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if n == 0 {
		if err != nil {
			return "", errors.New("cannot read file")
		}
		return "", errors.New("empty file")
	}

	contentType := http.DetectContentType(buf[:n])
	if !allowed[contentType] {
		return "", errors.New(invalidMessage)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", errors.New("failed to process file")
	}

	return contentType, nil
}

// SaveImage writes the uploaded file to targetDir with a generated filename of the form
// {prefix}_{random}.{ext}. It creates targetDir if it doesn't exist.
// Returns the full file path on disk.
func SaveImage(file io.Reader, targetDir, prefix, contentType string) (string, error) {
	ext := fileExtension(contentType)
	return saveUploadedFile(file, targetDir, prefix, ext)
}

// SavePDF writes the uploaded PDF to targetDir with a generated filename of the form
// {prefix}_{random}.pdf. It creates targetDir if it doesn't exist.
// Returns the full file path on disk.
func SavePDF(file io.Reader, targetDir, prefix string) (string, error) {
	return saveUploadedFile(file, targetDir, prefix, ".pdf")
}

func saveUploadedFile(file io.Reader, targetDir, prefix, ext string) (string, error) {
	randomStr, err := randstr.String(8, randstr.Alphanumeric)
	if err != nil {
		return "", errors.New("failed to generate filename")
	}

	return SaveNamedFile(file, targetDir, prefix+"_"+randomStr+ext)
}

// SaveNamedFile writes the uploaded file to targetDir under the exact given
// filename (callers generate collision-free names, e.g. UUIDs). It creates
// targetDir if it doesn't exist. Returns the full file path on disk.
func SaveNamedFile(file io.Reader, targetDir, filename string) (string, error) {
	return saveThroughStorage(file, targetDir, filename, storage.SaveOptions{})
}

// SavePrivateNamedFile writes a file that must never be exposed through a
// static mount. Both the directory and file are owner-only so access remains
// mediated by the authenticated download handler.
func SavePrivateNamedFile(file io.Reader, targetDir, filename string) (string, error) {
	return saveThroughStorage(file, targetDir, filename, storage.SaveOptions{Private: true})
}

// saveThroughStorage funnels every write into the storage backend and keeps
// returning an on-disk path so existing callers stay unchanged. The path is
// derived from the same root the backend uses, so path and key can never
// point at different files.
func saveThroughStorage(file io.Reader, targetDir, filename string, opts storage.SaveOptions) (string, error) {
	backend, dir, err := fileStore(targetDir)
	if err != nil {
		return "", errors.New("failed to create upload directory")
	}
	key, err := storage.Key(filename)
	if err != nil {
		return "", errors.New("failed to save file")
	}
	if _, err := backend.Save(context.Background(), key, file, opts); err != nil {
		return "", errors.New("failed to save file")
	}
	return filepath.Join(dir, filename), nil
}

// ServeFile serves a file from baseDir, validating the path stays within baseDir.
// If baseDir is relative (e.g. "public/uploads/..."), it is resolved via ResolvePublicDir.
// cacheControl sets the Cache-Control header (e.g. "private, max-age=86400" or "public, max-age=86400").
func ServeFile(w http.ResponseWriter, r *http.Request, baseDir, filename, cacheControl string) {
	servePublicFile(w, r, baseDir, filename, cacheControl)
}

// ServeImage serves an image file from baseDir, validating the path stays within baseDir.
// If baseDir is relative (e.g. "public/uploads/..."), it is resolved via ResolvePublicDir.
// cacheControl sets the Cache-Control header (e.g. "private, max-age=86400" or "public, max-age=86400").
func ServeImage(w http.ResponseWriter, r *http.Request, baseDir, filename, cacheControl string) {
	servePublicFile(w, r, baseDir, filename, cacheControl)
}

func servePublicFile(w http.ResponseWriter, r *http.Request, baseDir, filename, cacheControl string) {
	if err := ValidateFilename(filename); err != nil {
		http.NotFound(w, r)
		return
	}

	backend, _, err := fileStore(baseDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	key, err := storage.Key(filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	object, err := backend.Open(r.Context(), key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() {
		if err := object.Close(); err != nil {
			slog.Default().Error("file close error", slog.String("error", err.Error()))
		}
	}()

	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, filename, object.ModTime(), object)
}

// RemoveImage deletes a file from disk, logging any error.
func RemoveImage(filePath string) {
	if filePath == "" {
		return
	}
	backend, _, err := fileStore(filepath.Dir(filePath))
	if err != nil {
		slog.Default().Error("failed to remove file",
			slog.String("path", filePath),
			slog.String("error", err.Error()))
		return
	}
	key, err := storage.Key(filepath.Base(filePath))
	if err != nil {
		slog.Default().Error("failed to remove file",
			slog.String("path", filePath),
			slog.String("error", err.Error()))
		return
	}
	if err := backend.Remove(context.Background(), key); err != nil {
		slog.Default().Error("failed to remove file",
			slog.String("path", filePath),
			slog.String("error", err.Error()))
	}
}

// fileStore returns the storage backend for one upload directory plus the
// absolute directory it resolved to. Relative directories ("public/uploads/x")
// are resolved against the discovered public directory; absolute ones are used
// as given. This is the single place that answers "where do these bytes live",
// and every read, write and delete of an upload goes through the backend it
// returns — save, serve and remove therefore cannot disagree.
func fileStore(baseDir string) (storage.Backend, string, error) {
	dir := baseDir
	if !filepath.IsAbs(dir) {
		if pubDir, err := ResolvePublicDir(); err == nil {
			// Strip leading "public/" since pubDir already points to it
			rel := strings.TrimPrefix(dir, "public/")
			rel = strings.TrimPrefix(rel, "public")
			if rel != "" {
				dir = filepath.Join(pubDir, rel)
			} else {
				dir = pubDir
			}
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", errors.New("failed to resolve directory")
	}
	return storage.NewLocal(absDir), absDir, nil
}

// UploadsBackend returns the storage backend rooted at the uploads
// directory. Document features address objects inside it by key
// ({kind}/{tenant}/{name}) instead of assembling paths of their own.
func UploadsBackend() (storage.Backend, error) {
	backend, _, err := fileStore("public/uploads")
	return backend, err
}

// ValidateFilename rejects filenames containing path traversal characters.
func ValidateFilename(filename string) error {
	if filename == "" {
		return errors.New("filename required")
	}
	if strings.Contains(filename, "..") {
		return errors.New("invalid filename")
	}
	if filepath.Base(filename) != filename {
		return errors.New("invalid filename")
	}
	return nil
}

// ResolveStoredPath converts a stored URL path (e.g. "/uploads/avatars/global/file.jpg")
// to a validated filesystem path under the public directory, preventing path traversal.
// Uses ResolvePublicDir() to find the public directory regardless of working directory.
func ResolveStoredPath(publicDir, urlPath, requiredPrefix string) (string, error) {
	if !strings.HasPrefix(urlPath, requiredPrefix) {
		return "", errors.New("invalid path")
	}

	if strings.Contains(urlPath, "..") {
		return "", errors.New("invalid path")
	}

	// Resolve relative production paths, but preserve an explicit absolute root.
	// Tests and callers with isolated storage must not be redirected to the
	// process-wide discovered public directory.
	baseDir := publicDir
	if !filepath.IsAbs(baseDir) {
		if resolved, err := ResolvePublicDir(); err == nil {
			baseDir = resolved
		}
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", errors.New("failed to resolve directory")
	}

	filePath := filepath.Join(baseDir, strings.TrimPrefix(urlPath, "/"))
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", errors.New("failed to resolve path")
	}

	// The URL starts with / (e.g. /uploads/...) so after TrimPrefix("/") we get "uploads/..."
	// which is inside the public dir. But verify to prevent traversal.
	if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) {
		return "", errors.New("invalid path")
	}

	return absPath, nil
}

// CloseFile safely closes a file, logging any error.
func CloseFile(file io.Closer) {
	if err := file.Close(); err != nil {
		slog.Default().Error("file close error", slog.String("error", err.Error()))
	}
}

// fileExtension returns the extension derived from the detected content type.
// This ensures the stored file extension matches the actual image format.
func fileExtension(contentType string) string {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

var (
	resolvedPublicDir string
	resolvedPublicErr error
	resolvePublicOnce sync.Once
)

// ResolvePublicDir finds the "public" directory by searching from the working directory
// and executable path. Handles both running from the backend/ dir and from the repo root.
// Result is cached after the first call.
func ResolvePublicDir() (string, error) {
	resolvePublicOnce.Do(func() {
		var candidates []string

		if wd, err := os.Getwd(); err == nil {
			candidates = append(candidates, publicDirCandidates(wd)...)
		}

		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, publicDirCandidates(filepath.Dir(exe))...)
		}

		for _, candidate := range candidates {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			info, err := os.Stat(abs)
			if err == nil && info.IsDir() {
				resolvedPublicDir = abs
				return
			}
		}

		resolvedPublicErr = errors.New("public directory not found")
	})

	return resolvedPublicDir, resolvedPublicErr
}

// publicDirCandidates returns candidate paths for the public directory,
// walking up parent directories from base to find public/ or backend/public/.
func publicDirCandidates(base string) []string {
	var candidates []string
	for current := base; ; current = filepath.Dir(current) {
		candidates = append(candidates,
			filepath.Join(current, "public"),
			filepath.Join(current, "backend", "public"),
		)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return candidates
}
