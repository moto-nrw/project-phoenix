package common

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// detectDocumentContentType reads the first 512 bytes to detect the MIME
// type via magic bytes and validates it against the allowed document types.
func detectDocumentContentType(file io.ReadSeeker, filename string, kinds []ooxmlKind, invalidMessage string) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if n == 0 {
		if err != nil {
			return "", errors.New("cannot read file")
		}
		return "", errors.New("empty file")
	}

	contentType := http.DetectContentType(buf[:n])
	switch {
	case AllowedDocumentTypes[contentType]:
		// PDF or image, magic bytes are conclusive.
	case contentType == "application/zip":
		kind, ok := matchOOXMLKind(filename, kinds)
		if !ok || !isOOXML(file, kind) {
			return "", errors.New(invalidMessage)
		}
		contentType = kind.contentType
	default:
		return "", errors.New(invalidMessage)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", errors.New("failed to process file")
	}
	return contentType, nil
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
	switch err.Error() {
	case "file too large":
		return fmt.Errorf("Diese Datei ist zu groß. Erlaubt sind bis zu %d MB.", maxFileSize/(1024*1024))
	case "no file uploaded":
		return errors.New("Es wurde keine Datei ausgewählt.")
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
