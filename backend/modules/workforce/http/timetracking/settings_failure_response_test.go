package timetracking

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
)

type failedSettingsAudit struct{ workforce.TimeTrackingAuditLog }

func (failedSettingsAudit) ListAuditLog(context.Context, workforce.AuditLogListRequest) (*workforce.AuditLogPage, error) {
	return nil, errors.New("private audit settings read failure")
}
func TestAuditLogResponseHidesSettingsReadFailure(t *testing.T) {
	t.Parallel()
	rs := &StaffAdminResource{AuditLogService: failedSettingsAudit{}}
	response := httptest.NewRecorder()
	rs.getTimeTrackingAuditLog(response, httptest.NewRequest("GET", "/audit-log", nil))
	assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"code":"general.server"`)
	assert.NotContains(t, response.Body.String(), "private audit settings read failure")
}
