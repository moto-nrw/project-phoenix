package common

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/randstr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectContentType_JPEG(t *testing.T) {
	jpegBytes := make([]byte, 512)
	jpegBytes[0] = 0xFF
	jpegBytes[1] = 0xD8
	jpegBytes[2] = 0xFF

	reader := bytes.NewReader(jpegBytes)
	contentType, err := detectImageContentType(reader)

	assert.NoError(t, err)
	assert.Equal(t, "image/jpeg", contentType)
}

func TestDetectContentType_PNG(t *testing.T) {
	pngBytes := make([]byte, 512)
	copy(pngBytes, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	reader := bytes.NewReader(pngBytes)
	contentType, err := detectImageContentType(reader)

	assert.NoError(t, err)
	assert.Equal(t, "image/png", contentType)
}

func TestDetectContentType_Invalid(t *testing.T) {
	htmlBytes := make([]byte, 512)
	copy(htmlBytes, []byte("<!DOCTYPE html><html><body>Hello</body></html>"))

	reader := bytes.NewReader(htmlBytes)
	_, err := detectImageContentType(reader)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file type")
}

func TestDetectContentType_SmallFile(t *testing.T) {
	// A valid PNG smaller than 512 bytes should still be detected
	pngBytes := make([]byte, 16)
	copy(pngBytes, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	reader := bytes.NewReader(pngBytes)
	contentType, err := detectImageContentType(reader)

	assert.NoError(t, err)
	assert.Equal(t, "image/png", contentType)
}

func TestDetectContentType_Empty(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	_, err := detectImageContentType(reader)

	assert.Error(t, err)
}

func TestFileExtension_DerivedFromContentType(t *testing.T) {
	tests := []struct {
		contentType, expected string
	}{
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"application/octet-stream", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, fileExtension(tt.contentType))
	}
}

func TestGenerateRandomString(t *testing.T) {
	result, err := randstr.String(8, randstr.Alphanumeric)
	assert.NoError(t, err)
	assert.Len(t, result, 8)

	// Uniqueness
	result2, err := randstr.String(8, randstr.Alphanumeric)
	assert.NoError(t, err)
	assert.NotEqual(t, result, result2)
}

func TestValidateFilename(t *testing.T) {
	assert.NoError(t, ValidateFilename("test.jpg"))
	assert.NoError(t, ValidateFilename("123_abcdef.png"))

	assert.Error(t, ValidateFilename(""))
	assert.Error(t, ValidateFilename(".."))
	assert.Error(t, ValidateFilename("../etc/passwd"))
}

func TestResolveStoredPath_Valid(t *testing.T) {
	path, err := ResolveStoredPath("public", "/uploads/avatars/global/test.jpg", "/uploads/avatars/")
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestResolveStoredPath_InvalidPrefix(t *testing.T) {
	_, err := ResolveStoredPath("public", "/uploads/not-avatars/test.jpg", "/uploads/avatars/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path")
}

func TestResolveStoredPath_Traversal(t *testing.T) {
	_, err := ResolveStoredPath("public", "/uploads/avatars/../../etc/passwd", "/uploads/avatars/")
	assert.Error(t, err)
}

func TestCloseFile_Success(t *testing.T) {
	closer := &mockCloser{}
	CloseFile(closer)
	assert.True(t, closer.closed)
}

func TestCloseFile_WithError(t *testing.T) {
	closer := &mockCloser{err: assert.AnError}
	CloseFile(closer)
	assert.True(t, closer.closed)
}

func TestAllowedImageTypes_Correct(t *testing.T) {
	assert.True(t, AllowedImageTypes["image/jpeg"])
	assert.True(t, AllowedImageTypes["image/png"])
	assert.True(t, AllowedImageTypes["image/webp"])
	assert.False(t, AllowedImageTypes["image/gif"])
	assert.False(t, AllowedImageTypes["text/html"])
}

type mockCloser struct {
	closed bool
	err    error
}

func (m *mockCloser) Close() error {
	m.closed = true
	return m.err
}

// ---------------------------------------------------------------------------
// Tests for public functions: ParseImage, SaveImage, ServeImage, RemoveImage,
// ResolvePublicDir, publicDirCandidates, and modTime.
// ---------------------------------------------------------------------------

func createMultipartRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func makeJPEGBytes() []byte {
	b := make([]byte, 512)
	b[0] = 0xFF
	b[1] = 0xD8
	b[2] = 0xFF
	return b
}

func makePDFBytes() []byte {
	return []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")
}

func makeZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func TestParseDocumentWithLimits_ValidatesOOXMLPackage(t *testing.T) {
	validDOCX := makeZIP(t, map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   "<w:document/>",
	})
	req := createMultipartRequest(t, "document", "contract.docx", validDOCX)
	uploaded, err := ParseDocumentWithLimits(httptest.NewRecorder(), req, "document", 10<<20, 10<<20)
	require.NoError(t, err)
	defer func() { _ = uploaded.File.Close() }()
	assert.Equal(t, DocxContentType, uploaded.ContentType)

	plainZIP := makeZIP(t, map[string]string{"notes.txt": "not a Word document"})
	req = createMultipartRequest(t, "document", "renamed.docx", plainZIP)
	_, err = ParseDocumentWithLimits(httptest.NewRecorder(), req, "document", 10<<20, 10<<20)
	require.EqualError(t, err, invalidDocumentTypeMessage)
}

func TestParseImage_ValidJPEG(t *testing.T) {
	req := createMultipartRequest(t, "image", "photo.jpg", makeJPEGBytes())
	w := httptest.NewRecorder()

	uploaded, err := ParseImage(w, req, "image", 10<<20)
	require.NoError(t, err)
	defer func() { _ = uploaded.File.Close() }()

	assert.Equal(t, "image/jpeg", uploaded.ContentType)
	assert.NotEmpty(t, uploaded.Filename)
}

func TestParseImage_MissingField(t *testing.T) {
	req := createMultipartRequest(t, "other_field", "photo.jpg", makeJPEGBytes())
	w := httptest.NewRecorder()

	_, err := ParseImage(w, req, "image", 10<<20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no file uploaded")
}

func TestParseImage_ContentTypeSpoofed(t *testing.T) {
	// Content is HTML, but filename suggests JPEG
	htmlContent := make([]byte, 512)
	copy(htmlContent, []byte("<!DOCTYPE html><html><body>malicious</body></html>"))

	req := createMultipartRequest(t, "image", "evil.jpg", htmlContent)
	w := httptest.NewRecorder()

	_, err := ParseImage(w, req, "image", 10<<20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file type")
}

func TestParseImageWithLimits_AllowsExactFileSizeWithMultipartHeadroom(t *testing.T) {
	content := make([]byte, 1024)
	copy(content, makeJPEGBytes())
	req := createMultipartRequest(t, "image", "photo.jpg", content)
	w := httptest.NewRecorder()

	uploaded, err := ParseImageWithLimits(w, req, "image", int64(len(content)), int64(len(content)+1024))
	require.NoError(t, err)
	defer func() { _ = uploaded.File.Close() }()

	assert.Equal(t, "image/jpeg", uploaded.ContentType)
}

func TestParseImageWithLimits_RejectsFileOverAdvertisedLimit(t *testing.T) {
	content := make([]byte, 1025)
	copy(content, makeJPEGBytes())
	req := createMultipartRequest(t, "image", "photo.jpg", content)
	w := httptest.NewRecorder()

	_, err := ParseImageWithLimits(w, req, "image", 1024, int64(len(content)+1024))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")
}

func TestParsePDF_ValidDocument(t *testing.T) {
	req := createMultipartRequest(t, "document", "terms.pdf", makePDFBytes())
	w := httptest.NewRecorder()

	uploaded, err := ParsePDFWithLimits(w, req, "document", 10<<20, 10<<20)
	require.NoError(t, err)
	defer func() { _ = uploaded.File.Close() }()

	assert.Equal(t, "application/pdf", uploaded.ContentType)
	assert.Equal(t, "terms.pdf", uploaded.Filename)
}

func TestParsePDF_RejectsNonPDFContent(t *testing.T) {
	req := createMultipartRequest(t, "document", "terms.pdf", makeJPEGBytes())
	w := httptest.NewRecorder()

	_, err := ParsePDFWithLimits(w, req, "document", 10<<20, 10<<20)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Only PDF documents are allowed")
}

func TestParsePDFWithLimits_RejectsFileOverAdvertisedLimit(t *testing.T) {
	content := append(makePDFBytes(), bytes.Repeat([]byte("x"), 1024)...)
	req := createMultipartRequest(t, "document", "terms.pdf", content)
	w := httptest.NewRecorder()

	_, err := ParsePDFWithLimits(w, req, "document", int64(len(content)-1), int64(len(content)+1024))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")
}

func TestSaveImage_CreatesFileWithCorrectExtension(t *testing.T) {
	dir := t.TempDir()
	content := bytes.NewReader([]byte("fake image data"))

	path, err := SaveImage(content, dir, "avatar", "image/jpeg")
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(path, ".jpg"), "expected .jpg extension, got %s", path)

	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "file should exist on disk")
}

func TestSaveImage_GeneratesUniqueFilenames(t *testing.T) {
	dir := t.TempDir()

	path1, err := SaveImage(bytes.NewReader([]byte("data1")), dir, "img", "image/png")
	require.NoError(t, err)

	path2, err := SaveImage(bytes.NewReader([]byte("data2")), dir, "img", "image/png")
	require.NoError(t, err)

	assert.NotEqual(t, path1, path2)
}

func TestSaveImage_UnknownContentType(t *testing.T) {
	dir := t.TempDir()
	content := bytes.NewReader([]byte("pdf content"))

	path, err := SaveImage(content, dir, "doc", "application/pdf")
	require.NoError(t, err)

	// No extension for unknown content type
	assert.False(t, strings.HasSuffix(path, ".pdf"), "should not have .pdf extension")
	assert.True(t, strings.Contains(filepath.Base(path), "doc_"), "filename should have prefix")

	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "file should exist on disk")
}

func TestSavePDF_CreatesFileWithPDFExtension(t *testing.T) {
	dir := t.TempDir()
	content := makePDFBytes()

	path, err := SavePDF(bytes.NewReader(content), dir, "agb")
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(path, ".pdf"), "expected .pdf extension, got %s", path)
	assert.True(t, strings.Contains(filepath.Base(path), "agb_"), "filename should have prefix")

	savedContent, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, savedContent)
}

func TestSavePrivateNamedFile_UsesOwnerOnlyPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staff-documents")
	path, err := SavePrivateNamedFile(bytes.NewReader([]byte("sensitive")), dir, "document.pdf")
	require.NoError(t, err)

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm())
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm())
}

func TestServeImage_ValidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.jpg")
	require.NoError(t, os.WriteFile(filePath, makeJPEGBytes(), 0644))

	req := httptest.NewRequest(http.MethodGet, "/images/test.jpg", nil)
	w := httptest.NewRecorder()

	ServeImage(w, req, dir, "test.jpg", "public, max-age=86400")

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "public, max-age=86400", resp.Header.Get("Cache-Control"))
}

func TestServeImage_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/images/../../../etc/passwd", nil)
	w := httptest.NewRecorder()

	ServeImage(w, req, dir, "../../../etc/passwd", "public, max-age=86400")

	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestServeFile_ValidPDF(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "terms.pdf")
	require.NoError(t, os.WriteFile(filePath, makePDFBytes(), 0644))

	req := httptest.NewRequest(http.MethodGet, "/documents/terms.pdf", nil)
	w := httptest.NewRecorder()

	ServeFile(w, req, dir, "terms.pdf", "public, max-age=3600")

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "application/pdf", resp.Header.Get("Content-Type"))
}

func TestServeFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/documents/../../../etc/passwd", nil)
	w := httptest.NewRecorder()

	ServeFile(w, req, dir, "../../../etc/passwd", "public, max-age=3600")

	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestServeImage_NonExistent(t *testing.T) {
	dir := t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/images/doesnotexist.jpg", nil)
	w := httptest.NewRecorder()

	ServeImage(w, req, dir, "doesnotexist.jpg", "public, max-age=86400")

	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestRemoveImage_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "to_delete.jpg")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

	RemoveImage(filePath)

	_, err := os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "file should be deleted")
}

func TestRemoveImage_NonExistent(t *testing.T) {
	// Should not panic when file does not exist
	assert.NotPanics(t, func() {
		RemoveImage("/tmp/nonexistent_file_abc123.jpg")
	})
}

func TestRemoveImage_EmptyPath(t *testing.T) {
	// Should be a no-op, no panic
	assert.NotPanics(t, func() {
		RemoveImage("")
	})
}

func TestPublicDirCandidates(t *testing.T) {
	base := "/some/project/backend"
	candidates := publicDirCandidates(base)

	assert.NotEmpty(t, candidates)

	// Should contain "public" and "backend/public" relative to base and parents
	assert.Contains(t, candidates, filepath.Join(base, "public"))
	assert.Contains(t, candidates, filepath.Join(base, "backend", "public"))

	// Should also walk up to parent
	parent := filepath.Dir(base)
	assert.Contains(t, candidates, filepath.Join(parent, "public"))
	assert.Contains(t, candidates, filepath.Join(parent, "backend", "public"))
}

func TestModTime_ValidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "modtime_test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	f, err := os.Open(filePath)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	mt := modTime(f)
	assert.False(t, mt.IsZero(), "modification time should be non-zero for a real file")
}

func TestModTime_NilHandling(t *testing.T) {
	// Open a file, close it, then call modTime on the closed file descriptor.
	// This should return zero time (the error path) rather than panic.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "closed.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0644))

	f, err := os.Open(filePath)
	require.NoError(t, err)
	_ = f.Close() // intentionally close before calling modTime

	mt := modTime(f)
	assert.True(t, mt.IsZero(), "should return zero time for closed file descriptor")
}
