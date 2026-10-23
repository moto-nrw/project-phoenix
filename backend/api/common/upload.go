package common

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
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
		return nil, ErrUploadTooLarge
	}

	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, ErrUploadMissing
	}
	if header.Size > maxFileSize {
		_ = file.Close()
		return nil, ErrUploadTooLarge
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
func SaveImage(file io.Reader, targetDir, prefix, contentType string, uploads UploadStorage) (string, error) {
	ext := fileExtension(contentType)
	return saveUploadedFile(file, targetDir, prefix, ext, uploads, false)
}

// SavePrivateImage preserves SaveImage's filename format while storing the
// image and its destination directory with owner-only permissions.
func SavePrivateImage(file io.Reader, targetDir, prefix, contentType string, uploads UploadStorage) (string, error) {
	return saveUploadedFile(file, targetDir, prefix, fileExtension(contentType), uploads, true)
}

// SavePDF writes the uploaded PDF to targetDir with a generated filename of the form
// {prefix}_{random}.pdf. It creates targetDir if it doesn't exist.
// Returns the full file path on disk.
func SavePDF(file io.Reader, targetDir, prefix string, uploads UploadStorage) (string, error) {
	return saveUploadedFile(file, targetDir, prefix, ".pdf", uploads, false)
}

func saveUploadedFile(file io.Reader, targetDir, prefix, ext string, uploads UploadStorage, private bool) (string, error) {
	randomStr, err := securityruntime.UploadFilenameSuffix()
	if err != nil {
		return "", errors.New("failed to generate filename")
	}

	filename := prefix + "_" + randomStr + ext
	if private {
		return SavePrivateNamedFile(file, targetDir, filename, uploads)
	}
	return SaveNamedFile(file, targetDir, filename, uploads)
}

// SaveNamedFile writes the uploaded file to targetDir under the exact given
// filename (callers generate collision-free names, e.g. UUIDs). It creates
// targetDir if it doesn't exist. Returns the full file path on disk.
func SaveNamedFile(file io.Reader, targetDir, filename string, uploads UploadStorage) (string, error) {
	return uploads.SaveNamed(file, targetDir, filename, false)
}

// SavePrivateNamedFile writes a file that must never be exposed through a
// static mount. Both the directory and file are owner-only so access remains
// mediated by the authenticated download handler.
func SavePrivateNamedFile(file io.Reader, targetDir, filename string, uploads UploadStorage) (string, error) {
	return uploads.SaveNamed(file, targetDir, filename, true)
}

// ServeFile serves a file from baseDir, validating the path stays within baseDir.
// If baseDir is relative (e.g. "public/uploads/..."), it is resolved via ResolvePublicDir.
// cacheControl sets the Cache-Control header (e.g. "private, max-age=86400" or "public, max-age=86400").
func ServeFile(w http.ResponseWriter, r *http.Request, baseDir, filename, cacheControl string, uploads UploadStorage) {
	servePublicFile(w, r, baseDir, filename, cacheControl, uploads)
}

// ServeImage serves an image file from baseDir, validating the path stays within baseDir.
// If baseDir is relative (e.g. "public/uploads/..."), it is resolved via ResolvePublicDir.
// cacheControl sets the Cache-Control header (e.g. "private, max-age=86400" or "public, max-age=86400").
func ServeImage(w http.ResponseWriter, r *http.Request, baseDir, filename, cacheControl string, uploads UploadStorage) {
	servePublicFile(w, r, baseDir, filename, cacheControl, uploads)
}

func servePublicFile(w http.ResponseWriter, r *http.Request, baseDir, filename, cacheControl string, uploads UploadStorage) {
	if err := ValidateFilename(filename); err != nil {
		RenderStatus(w, r, http.StatusNotFound, "")
		return
	}

	file, modifiedAt, err := uploads.Open(r.Context(), baseDir, filename)
	if err != nil {
		RenderStatus(w, r, http.StatusNotFound, "")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Error("file close error", slog.String("error", err.Error()))
		}
	}()

	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, filename, modifiedAt, file)
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

// Multipart upload refusal identities; diagnostics are part of the wire contract.
var (
	ErrUploadTooLarge = errors.New("file too large")
	ErrUploadMissing  = errors.New("no file uploaded")
)
