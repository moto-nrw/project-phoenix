package importapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenValidatedUploadFileClassifiesTruncatedMultipartAsUnreadable(t *testing.T) {
	t.Parallel()

	var got Failure
	resource := &Resource{runtime: Runtime{
		Failure: func(_ http.ResponseWriter, _ *http.Request, failure Failure) {
			got = failure
		},
	}}
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(
		"--upload\r\nContent-Disposition: form-data; name=\"file\"; filename=\"list.csv\"\r\n\r\nVorname",
	))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=upload")

	_, _, _, ok := resource.openValidatedUploadFile(httptest.NewRecorder(), req)

	assert.False(t, ok)
	assert.Equal(t, codeImportFileUnreadable, got.Code)
	assert.Equal(t, http.StatusBadRequest, got.Status)
}
