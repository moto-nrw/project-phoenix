package enrollmenthttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// countingBody counts the bytes a handler reads from the request body.
type countingBody struct {
	io.Reader
	read int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += int64(n)
	return n, err
}

type unknownSchool struct{}

func (unknownSchool) GetSchoolBySlug(context.Context, string) (*PublicSchool, error) {
	return nil, errors.New("no such school")
}

type unknownHolidayLink struct {
	capability.HolidayRegistrations
}

func (unknownHolidayLink) ResolveHolidayRegistrationLink(context.Context, string) (capability.HolidayRegistrationLink, error) {
	return capability.HolidayRegistrationLink{}, capability.ErrHolidayRegistrationNotFound
}

type unusedProofUploads struct{ HolidayProofUploads }

type unusedUploadService struct{ UploadService }

// TestPublicUploadsRefuseUnknownLinksBeforeReadingTheBody: a public upload
// with an unknown school or link reads no file. Otherwise anyone could make
// the server read and buffer 10 MB per request without a valid link.
func TestPublicUploadsRefuseUnknownLinksBeforeReadingTheBody(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	require.NoError(t, writer.WriteField("phase_id", "1"))
	part, err := writer.CreateFormFile("file", "Bescheid.pdf")
	require.NoError(t, err)
	_, err = part.Write(append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte{' '}, 8<<20)...))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	enrollment := &Resource{SchoolService: unknownSchool{}, Uploads: unusedUploadService{}, UploadFiles: func(string) (UploadFileStore, error) { return nil, errors.New("unused") }}
	holiday := NewHolidayRegistrationResource(unknownHolidayLink{}, unusedProofUploads{}, settingsFailureCaptcha{}, nil)
	for _, tc := range []struct {
		name, path, code string
		router           http.Handler
	}{
		{"enrollment form", "/unbekannt/uploads", "enrollment.form_not_found", enrollment.Router(func(chi.Router) {})},
		{"holiday registration", "/unbekannter-link/uploads", "enrollment.holiday_registration_not_found", holiday.Router()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &countingBody{Reader: bytes.NewReader(payload.Bytes())}
			req := httptest.NewRequest(http.MethodPost, tc.path, body)
			req = req.WithContext(testpkg.WithTestTenantRuntime(t, req.Context()))
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := httptest.NewRecorder()

			tc.router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			var response struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			assert.Equal(t, tc.code, response.Code)
			assert.Less(t, body.read, int64(64<<10), "the body of an unknown link is not read")
		})
	}
}
