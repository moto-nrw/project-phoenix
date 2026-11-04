package timetracking

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// downloadOnce is a dispatch whose one link opens once in its school.
type downloadOnce struct {
	payrollDispatchOff
	tenantID int64
	token    string
	spent    bool
}

func (d *downloadOnce) Download(_ context.Context, tenantID int64, token string) (workforce.PayrollDownload, error) {
	if d.spent || tenantID != d.tenantID || token != d.token {
		return workforce.PayrollDownload{}, workforce.ErrPayrollDownloadUnavailable
	}
	d.spent = true
	return workforce.PayrollDownload{Filename: "zeiterfassung_alle_2026-09.csv", ContentType: "text/csv; charset=utf-8", Data: []byte("Zeiten")}, nil
}

func TestPayrollDownloadOpensOnceAndOnlyByPost(t *testing.T) {
	t.Parallel()
	dispatch := &downloadOnce{tenantID: 4805, token: "abcdefghijklmnopqrstuvwxyz"}
	router := NewPayrollDownloadResource(dispatch, nil).PublicRouter()
	request := func(method, link string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(method, "/"+link, nil))
		return recorder
	}

	assert.Equal(t, http.StatusMethodNotAllowed, request(http.MethodGet, "4805.abcdefghijklmnopqrstuvwxyz").Code,
		"a mail scanner's GET must not spend the link")
	assert.False(t, dispatch.spent)

	first := request(http.MethodPost, "4805.abcdefghijklmnopqrstuvwxyz")
	require.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, "Zeiten", first.Body.String())
	assert.Equal(t, `attachment; filename=zeiterfassung_alle_2026-09.csv`, first.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", first.Header().Get("Cache-Control"))

	for _, link := range []string{"4805.abcdefghijklmnopqrstuvwxyz", "4806.abcdefghijklmnopqrstuvwxyz", "kein-link", "0.abc", "4805."} {
		response := request(http.MethodPost, link)
		assert.Equal(t, http.StatusNotFound, response.Code, link)
		assert.Contains(t, response.Body.String(), `"code":"workforce.payroll_download_unavailable"`, link)
	}
}
