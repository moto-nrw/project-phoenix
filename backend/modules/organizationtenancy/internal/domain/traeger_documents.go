package domain

import (
	"archive/zip"
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Shared documents of a Träger (#4692). A file is admitted by its name and
// its content together: the extension names the type, and the first bytes
// (or, for Word, Excel and PowerPoint, the part of the Office archive that makes it
// one) must be that type. The name alone never decides.

// The refusals of an upload; the application maps them onto the public
// contract.
var (
	ErrDocumentFileMissing = errors.New("no file uploaded")
	ErrDocumentTooLarge    = errors.New("the file exceeds the limit")
	ErrDocumentFileType    = errors.New("the file type is not admitted")
)

// TraegerDocumentWrite is a checked document the store adds.
type TraegerDocumentWrite struct {
	Title       string
	FileName    string
	ContentType string
	Data        []byte
}

// DocumentMaxBytes is the largest file a Träger shares.
const DocumentMaxBytes = 10 << 20

const (
	contentTypePDF  = "application/pdf"
	contentTypePNG  = "image/png"
	contentTypeJPEG = "image/jpeg"
	contentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	contentTypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	contentTypePPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
)

// traegerDocumentTypes maps an admitted extension to its content type.
var traegerDocumentTypes = map[string]string{
	".pdf": contentTypePDF, ".png": contentTypePNG, ".jpg": contentTypeJPEG, ".jpeg": contentTypeJPEG,
	".docx": contentTypeDOCX, ".xlsx": contentTypeXLSX, ".pptx": contentTypePPTX,
}

// TraegerDocumentContentType checks the file name and the content of an
// upload and returns the content type the document is stored with.
func TraegerDocumentContentType(fileName string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrDocumentFileMissing
	}
	if len(data) > DocumentMaxBytes {
		return "", ErrDocumentTooLarge
	}
	contentType, ok := traegerDocumentTypes[strings.ToLower(filepath.Ext(fileName))]
	if !ok || !contentMatches(contentType, data) {
		return "", ErrDocumentFileType
	}
	return contentType, nil
}

func contentMatches(contentType string, data []byte) bool {
	switch contentType {
	case contentTypePDF:
		return bytes.HasPrefix(data, []byte("%PDF-"))
	case contentTypePNG:
		return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	case contentTypeJPEG:
		return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
	case contentTypeDOCX:
		return officeArchiveHas(data, "word/document.xml")
	case contentTypeXLSX:
		return officeArchiveHas(data, "xl/workbook.xml")
	case contentTypePPTX:
		return officeArchiveHas(data, "ppt/presentation.xml")
	}
	return false
}

// officeArchiveHas reports whether data is a ZIP archive with the given part
// and the Office content-type list, as every Word, Excel and PowerPoint file is.
func officeArchiveHas(data []byte, part string) bool {
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return false
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	var hasPart, hasTypes bool
	for _, file := range archive.File {
		switch file.Name {
		case part:
			hasPart = true
		case "[Content_Types].xml":
			hasTypes = true
		}
	}
	return hasPart && hasTypes
}

// TraegerDocumentFileName keeps the base name of an uploaded file, without
// control characters, at most 255 bytes long and with its extension.
func TraegerDocumentFileName(fileName string) string {
	name := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' || r == '"' {
			return -1
		}
		return r
	}, filepath.Base(strings.ReplaceAll(fileName, "\\", "/")))
	name = strings.TrimSpace(name)
	if len(name) <= 255 {
		return name
	}
	ext := filepath.Ext(name)
	stem := name[:255-len(ext)]
	for !utf8.ValidString(stem) {
		stem = stem[:len(stem)-1]
	}
	return stem + ext
}

// The audiences of a release, as the public contract names them.
const (
	AudienceParents = "parents"
	AudienceSchools = "schools"
)

// ValidTraegerDocumentRelease reports whether a release has a shape its
// audience admits: parents of all schools, a school or a group of a school;
// schools all or one, never a group.
func ValidTraegerDocumentRelease(audience string, schoolID, groupID *int64) bool {
	if schoolID != nil && *schoolID <= 0 || groupID != nil && *groupID <= 0 {
		return false
	}
	switch audience {
	case AudienceParents:
		return groupID == nil || schoolID != nil
	case AudienceSchools:
		return groupID == nil
	}
	return false
}
