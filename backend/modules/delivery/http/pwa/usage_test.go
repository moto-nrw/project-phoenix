package pwa_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/http/pwa"
	"github.com/stretchr/testify/assert"
)

type usageRecorder struct {
	reported []int64
	fail     error
}

func (u *usageRecorder) ReportStaff(_ context.Context, accountID int64) error {
	u.reported = append(u.reported, accountID)
	return u.fail
}

func serveUsage(usage pwa.Usage, accountID int64) (int, string) {
	message := ""
	handler := pwa.ReportUsage(usage, pwa.UsageRuntime{
		AccountID:    func(context.Context) int64 { return accountID },
		Unauthorized: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
		Failed: func(w http.ResponseWriter, _ *http.Request, text string, _ error) {
			message = text
			w.WriteHeader(http.StatusInternalServerError)
		},
		NoContent: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/usage", nil))
	return recorder.Code, message
}

func TestReportUsageRecordsTheCallersSession(t *testing.T) {
	t.Parallel()
	usage := &usageRecorder{}
	status, _ := serveUsage(usage, 42)
	assert.Equal(t, http.StatusNoContent, status)
	assert.Equal(t, []int64{42}, usage.reported)

	status, _ = serveUsage(usage, 0)
	assert.Equal(t, http.StatusUnauthorized, status, "no session, no report")
	assert.Equal(t, []int64{42}, usage.reported)

	status, message := serveUsage(&usageRecorder{fail: errors.New("down")}, 42)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "App-Nutzung konnte nicht gespeichert werden.", message)
}
