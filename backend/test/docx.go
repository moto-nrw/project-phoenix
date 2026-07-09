package test

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

// ReadDocxDocumentXML extracts word/document.xml from DOCX bytes for content
// assertions in export tests.
func ReadDocxDocumentXML(tb testing.TB, data []byte) string {
	tb.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		tb.Fatalf("zip reader error = %v", err)
	}
	for _, entry := range reader.File {
		if entry.Name != "word/document.xml" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			tb.Fatalf("open document.xml error = %v", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				tb.Errorf("close document.xml error = %v", err)
			}
		}()
		content, err := io.ReadAll(rc)
		if err != nil {
			tb.Fatalf("read document.xml error = %v", err)
		}
		return string(content)
	}
	tb.Fatal("DOCX missing word/document.xml")
	return ""
}
