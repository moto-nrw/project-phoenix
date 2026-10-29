package enrollmenthttp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHolidayProofContentMismatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, content string }{
		{"executable renamed to PDF", "MZ disguised executable"},
		{"PNG renamed to PDF", "\x89PNG\r\n\x1a\nimage content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "Bescheid.pdf")
			require.NoError(t, err)
			_, err = part.Write([]byte(tc.content))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			req := httptest.NewRequest(http.MethodPost, "/proofs", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			// Rejection must happen before link resolution or storage is reached.
			(&HolidayRegistrationResource{}).uploadProof(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code)
			var response struct {
				Code    string         `json:"code"`
				Details map[string]any `json:"details"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Equal(t, "enrollment.upload_content_mismatch", response.Code)
			require.Equal(t, "Bescheid.pdf", response.Details["filename"])
		})
	}
}

func TestHolidayProofMatchingContentAccepted(t *testing.T) {
	t.Parallel()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Bescheid.pdf")
	require.NoError(t, err)
	content := []byte("%PDF-1.4\nproof")
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/proofs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	upload, err := parseHolidayProof(httptest.NewRecorder(), req)
	require.NoError(t, err)
	require.Equal(t, "Bescheid.pdf", upload.Filename)
	require.Equal(t, "application/pdf", upload.ContentType)
	require.Equal(t, content, upload.Content)
}

func TestHolidayProofUnsupportedDocumentIsNotContentMismatch(t *testing.T) {
	t.Parallel()
	var document bytes.Buffer
	archive := zip.NewWriter(&document)
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		part, err := archive.Create(name)
		require.NoError(t, err)
		_, err = part.Write([]byte("<xml/>"))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Bescheid.docx")
	require.NoError(t, err)
	_, err = part.Write(document.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/proofs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	(&HolidayRegistrationResource{}).uploadProof(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var response struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "enrollment.upload_invalid", response.Code)
}
