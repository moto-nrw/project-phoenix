package common

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsedUploadTempFileGoesWithTheRequest: a part above the memory
// threshold spills to a temporary file, which goes when the request context
// ends, even though routers parse a copy of the request.
func TestParsedUploadTempFileGoesWithTheRequest(t *testing.T) {
	t.Parallel()
	body, contentType := spillingUpload(t)
	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/uploads", body)
	req.Header.Set("Content-Type", contentType)

	uploaded, err := ParseDocumentWithLimits(httptest.NewRecorder(), req.WithContext(ctx), "file", 8<<20, 9<<20)
	require.NoError(t, err)
	spilled, ok := uploaded.File.(*os.File)
	require.True(t, ok, "a part above the memory threshold is a temporary file")
	name := spilled.Name()
	require.NoError(t, uploaded.File.Close())
	_, err = os.Stat(name)
	require.NoError(t, err, "the file stays while the request runs")

	cancel()

	assertTempFileGoes(t, name)
}

// TestRefusedUploadTempFileGoesWithTheRequest: a malformed query refuses the
// upload after the body spilled to disk; the file still goes.
func TestRefusedUploadTempFileGoesWithTheRequest(t *testing.T) {
	t.Parallel()
	body, contentType := spillingUpload(t)
	ctx, cancel := context.WithCancel(t.Context())
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/uploads?x=%zz", body)
	req.Header.Set("Content-Type", contentType)

	_, err := ParseDocumentWithLimits(httptest.NewRecorder(), req, "file", 8<<20, 9<<20)
	require.Error(t, err)
	require.NotNil(t, req.MultipartForm, "the form is parsed despite the query error")
	headers := req.MultipartForm.File["file"]
	require.Len(t, headers, 1)
	file, err := headers[0].Open()
	require.NoError(t, err)
	spilled, ok := file.(*os.File)
	require.True(t, ok, "a part above the memory threshold is a temporary file")
	name := spilled.Name()
	require.NoError(t, file.Close())

	cancel()

	assertTempFileGoes(t, name)
}

func spillingUpload(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Bescheid.pdf")
	require.NoError(t, err)
	_, err = part.Write(append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte{' '}, 2*maxMultipartMemory)...))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &body, writer.FormDataContentType()
}

func assertTempFileGoes(t *testing.T, name string) {
	t.Helper()
	assert.Eventually(t, func() bool {
		_, statErr := os.Stat(name)
		return errors.Is(statErr, fs.ErrNotExist)
	}, 2*time.Second, 10*time.Millisecond, "the temporary file goes with the request")
}
