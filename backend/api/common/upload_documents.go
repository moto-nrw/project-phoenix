package common

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/render"
)

// DocxContentType is the DOCX MIME type. http.DetectContentType cannot
// produce it — DOCX is an OOXML ZIP container, so magic bytes only prove
// application/zip and the package contents disambiguate it.
const DocxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// XlsxContentType and PptxContentType are the other two OOXML containers the
// school file storage accepts (#2596). Same detection: ZIP magic bytes plus
// the package part that only that format carries.
const (
	XlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	PptxContentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

// AllowedDocumentTypes maps MIME types detected by http.DetectContentType to
// allowed document uploads (staff documents, #1424). OOXML containers are
// handled separately — see ooxmlKind.
var AllowedDocumentTypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/jpg":       true,
}

// ooxmlKind describes one OOXML container: the extension the upload must
// carry, the package part that proves the format, and the MIME type recorded
// for it.
type ooxmlKind struct {
	extension   string
	part        string
	contentType string
}

var (
	docxKind = ooxmlKind{extension: ".docx", part: "word/document.xml", contentType: DocxContentType}
	xlsxKind = ooxmlKind{extension: ".xlsx", part: "xl/workbook.xml", contentType: XlsxContentType}
	pptxKind = ooxmlKind{extension: ".pptx", part: "ppt/presentation.xml", contentType: PptxContentType}
)

// maxMultipartMemory is the part of a multipart body held in memory; the
// rest of a file goes to a temporary file.
const maxMultipartMemory = 1 << 20

// documentUploadKinds is what child and staff documents accept.
var documentUploadKinds = []ooxmlKind{docxKind}

// officeUploadKinds is what the school file storage accepts.
var officeUploadKinds = []ooxmlKind{docxKind, xlsxKind, pptxKind}

const invalidDocumentTypeMessage = "Diese Datei ist nicht erlaubt. Erlaubt sind PDF, DOCX, PNG und JPEG."

const invalidOfficeFileTypeMessage = "Diese Datei ist nicht erlaubt. Erlaubt sind PDF, DOCX, XLSX, PPTX, PNG und JPEG."

// ParseDocumentWithLimits parses a multipart document upload and validates
// the content via magic bytes: PDF, PNG and JPEG directly, DOCX as a ZIP
// container with the required OOXML parts (a bare .zip is rejected).
// Enforces the advertised file-size cap separately from the multipart body
// cap. The caller must close UploadedFile.File.
func ParseDocumentWithLimits(w http.ResponseWriter, r *http.Request, fieldName string, maxFileSize, maxBodySize int64) (*UploadedFile, error) {
	return parseValidatedUpload(w, r, fieldName, maxFileSize, maxBodySize, documentUploadKinds, invalidDocumentTypeMessage)
}

// ParseOfficeFileWithLimits is ParseDocumentWithLimits with the wider set of
// office containers the school file storage accepts (#2596): DOCX, XLSX and
// PPTX. Nothing executable or scriptable (HTML, SVG, bare ZIP) passes.
func ParseOfficeFileWithLimits(w http.ResponseWriter, r *http.Request, fieldName string, maxFileSize, maxBodySize int64) (*UploadedFile, error) {
	return parseValidatedUpload(w, r, fieldName, maxFileSize, maxBodySize, officeUploadKinds, invalidOfficeFileTypeMessage)
}

func parseValidatedUpload(w http.ResponseWriter, r *http.Request, fieldName string, maxFileSize, maxBodySize int64, kinds []ooxmlKind, invalidMessage string) (*UploadedFile, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	// Parts beyond the memory threshold spill to temporary files. The server
	// cancels the request context once the handler returns; the files go
	// then. net/http's own cleanup misses them, because routers parse a copy
	// of the request (WithContext).
	// The form is set even when the query string fails to parse.
	err := r.ParseMultipartForm(min(maxBodySize, maxMultipartMemory))
	removeMultipartFilesAfter(r)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return nil, uploadRefusal{code: CodeFilesFileTooLarge, text: "file too large"}
		}
		return nil, uploadRefusal{code: CodeFilesFileUnreadable, text: "cannot read upload"}
	}

	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, uploadRefusal{code: CodeFilesFileMissing, text: "no file uploaded"}
	}
	if header.Size > maxFileSize {
		_ = file.Close()
		return nil, uploadRefusal{code: CodeFilesFileTooLarge, text: "file too large"}
	}

	contentType, err := detectDocumentContentType(file, header.Filename, kinds, invalidMessage)
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

// removeMultipartFilesAfter removes the temporary files of the parsed form
// when the request context ends.
func removeMultipartFilesAfter(r *http.Request) {
	if form := r.MultipartForm; form != nil {
		context.AfterFunc(r.Context(), func() { _ = form.RemoveAll() })
	}
}

// detectDocumentContentType reads the first 512 bytes to detect the MIME
// type via magic bytes and validates it against the allowed document types.
func detectDocumentContentType(file io.ReadSeeker, filename string, kinds []ooxmlKind, invalidMessage string) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if n == 0 {
		if err != nil {
			return "", uploadRefusal{code: CodeFilesFileUnreadable, text: "cannot read file"}
		}
		return "", uploadRefusal{code: CodeFilesFileUnreadable, text: "empty file"}
	}

	contentType := http.DetectContentType(buf[:n])
	switch {
	case AllowedDocumentTypes[contentType]:
		// PDF or image, magic bytes are conclusive.
	case contentType == "application/zip":
		kind, ok := matchOOXMLKind(filename, kinds)
		if !ok || !isOOXML(file, kind) {
			return "", uploadRefusal{code: CodeFilesFileTypeNotAllowed, text: invalidMessage}
		}
		contentType = kind.contentType
	default:
		return "", uploadRefusal{code: CodeFilesFileTypeNotAllowed, text: invalidMessage}
	}
	// The original name is stored and served as the download name, so a PDF
	// named brief.html would be saved by the browser as HTML.
	if !ExtensionMatchesContent(filename, contentType) {
		return "", uploadRefusal{code: CodeFilesFileTypeNotAllowed, text: invalidMessage}
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", uploadRefusal{code: CodeFilesFileUnreadable, text: "failed to process file"}
	}
	return contentType, nil
}

// ExtensionMatchesContent reports whether the file name's extension names
// the detected content type. A PDF renamed to .png or .html, or a file
// without a known extension, is refused even though the bytes alone pass.
func ExtensionMatchesContent(filename, contentType string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return contentType == "application/pdf"
	case ".png":
		return contentType == "image/png"
	case ".jpg", ".jpeg":
		return contentType == "image/jpeg" || contentType == "image/jpg"
	case docxKind.extension:
		return contentType == DocxContentType
	case xlsxKind.extension:
		return contentType == XlsxContentType
	case pptxKind.extension:
		return contentType == PptxContentType
	default:
		return false
	}
}

func matchOOXMLKind(filename string, kinds []ooxmlKind) (ooxmlKind, bool) {
	lower := strings.ToLower(filename)
	for _, kind := range kinds {
		if strings.HasSuffix(lower, kind.extension) {
			return kind, true
		}
	}
	return ooxmlKind{}, false
}

// isOOXML verifies the minimum OOXML structure that distinguishes an Office
// document from an arbitrary ZIP archive. Uploads are capped at the
// documented upload limit, so reading the container here is bounded.
func isOOXML(file io.ReadSeeker, kind ooxmlKind) bool {
	if _, err := file.Seek(0, 0); err != nil {
		return false
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return false
	}
	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return false
	}

	parts := make(map[string]struct{}, len(reader.File))
	for _, entry := range reader.File {
		parts[entry.Name] = struct{}{}
	}
	_, hasContentTypes := parts["[Content_Types].xml"]
	_, hasPart := parts[kind.part]
	return hasContentTypes && hasPart
}

// uploadRefusal is a rejected upload: the code the form explains (#2517) and
// the diagnostic text GermanUploadError restates.
type uploadRefusal struct {
	code string
	text string
}

func (e uploadRefusal) Error() string { return e.text }

// IsUploadTypeNotAllowed reports a content-type refusal from the upload parser.
func IsUploadTypeNotAllowed(err error) bool {
	var refusal uploadRefusal
	return errors.As(err, &refusal) && refusal.code == CodeFilesFileTypeNotAllowed
}

func uploadLimitMB(maxFileSize int64) int64 {
	const megabyte = 1024 * 1024
	if maxFileSize <= 0 {
		return 0
	}
	return (maxFileSize-1)/megabyte + 1
}

// ErrorUpload renders a refused upload as a 400 with its code; the size
// refusal names the limit in details. Other errors keep the plain 400.
func ErrorUpload(err error, maxFileSize int64) render.Renderer {
	var refusal uploadRefusal
	if !errors.As(err, &refusal) {
		return ErrorInvalidRequest(err)
	}
	shown := GermanUploadError(err, maxFileSize)
	if refusal.code == CodeFilesFileTooLarge {
		return ErrorInvalidRequestWithDetails(shown, refusal.code, map[string]any{"max_mb": uploadLimitMB(maxFileSize)})
	}
	return ErrorInvalidRequestWithCode(shown, refusal.code)
}

// GermanUploadError restates the size and shape rejections of
// ParseDocumentWithLimits / ParseOfficeFileWithLimits in German, naming the
// limit the caller ran into.
//
// The type rejections are already German and carry the allowed formats; the
// two remaining ones ("file too large", "no file uploaded") come from the
// multipart layer and would otherwise reach a parent or a school employee in
// English. The message says the limit in MB, because "zu groß" without a
// number leaves the person guessing what to try next.
func GermanUploadError(err error, maxFileSize int64) error {
	if err == nil {
		return nil
	}
	var refusal uploadRefusal
	errors.As(err, &refusal)
	switch {
	case errors.Is(err, ErrUploadTooLarge), refusal.code == CodeFilesFileTooLarge:
		return fmt.Errorf("diese Datei ist zu groß. Erlaubt sind bis zu %d MB", uploadLimitMB(maxFileSize))
	case errors.Is(err, ErrUploadMissing), refusal.code == CodeFilesFileMissing:
		return errors.New("es wurde keine Datei ausgewählt")
	default:
		return err
	}
}

// DocumentFileExtension returns the canonical stored-file extension for a
// validated document content type.
func DocumentFileExtension(contentType string) string {
	switch contentType {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case DocxContentType:
		return ".docx"
	case XlsxContentType:
		return ".xlsx"
	case PptxContentType:
		return ".pptx"
	default:
		return ""
	}
}
