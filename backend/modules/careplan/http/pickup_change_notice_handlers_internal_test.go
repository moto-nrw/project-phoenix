package careplanhttp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	permissions "github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	careplan "github.com/moto-ogs/project-phoenix/modules/careplan"
	sessiontoken "github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

type pickupNoticesDouble struct {
	reader, student int64
	observed        []careplan.PickupChangeNoticeRevision
	notices         []careplan.PickupChangeTeamNotice
}

func (d *pickupNoticesDouble) ListUnreadPickupChangeNotices(_ context.Context, reader int64) ([]careplan.PickupChangeTeamNotice, error) {
	d.reader = reader
	return d.notices, nil
}

func (d *pickupNoticesDouble) MarkPickupChangeNoticesRead(_ context.Context, reader, student int64, observed []careplan.PickupChangeNoticeRevision) ([]careplan.PickupChangeTeamNotice, error) {
	d.reader, d.student, d.observed = reader, student, observed
	return []careplan.PickupChangeTeamNotice{}, nil
}

func pickupNoticeRequest(method, target string, accountID int, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: accountID}))
}

// The header reads only the caller's own markers, with the child's first
// name and the day as YYYY-MM-DD; opening a child clears that child's.
func TestPickupChangeNoticeRoutesServeTheCallersMarkers(t *testing.T) {
	t.Parallel()
	informed := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	notices := &pickupNoticesDouble{notices: []careplan.PickupChangeTeamNotice{{
		RequestID: 5, StudentID: 77, StudentFirstName: "Mia", Date: calendar.NewDate(2026, 11, 13), PickupTime: "15:00", InformedAt: informed,
	}}}
	rs := &careplanhttp.Resource{ResourceConfig: careplanhttp.ResourceConfig{PickupChangeNotices: notices}}

	w := httptest.NewRecorder()
	serveRoute(t, rs, w, pickupNoticeRequest(http.MethodGet, "/students/pickup-change-notices", 41, ""), permissions.UsersRead)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(41), notices.reader)
	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, map[string]any{
		"request_id": "5", "student_id": "77", "student_first_name": "Mia", "date": "2026-11-13",
		"pickup_time": "15:00", "withdrawn": false, "informed_at": "2026-11-12T08:00:00Z",
	}, body.Data[0])

	w = httptest.NewRecorder()
	serveRoute(t, rs, w, pickupNoticeRequest(http.MethodPost, "/students/77/pickup-change-notices/read", 41,
		`{"notices":[{"request_id":"5","informed_at":"2026-11-12T08:00:00.123456Z"}]}`), permissions.UsersRead)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(77), notices.student)
	assert.Equal(t, []careplan.PickupChangeNoticeRevision{{RequestID: 5, InformedAt: time.Date(2026, 11, 12, 8, 0, 0, 123456000, time.UTC)}}, notices.observed)
	assert.JSONEq(t, `[]`, string(mustData(t, w)))

	w = httptest.NewRecorder()
	serveRoute(t, rs, w, pickupNoticeRequest(http.MethodPost, "/students/x/pickup-change-notices/read", 41, `{"notices":[]}`), permissions.UsersRead)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	serveRoute(t, rs, w, pickupNoticeRequest(http.MethodPost, "/students/77/pickup-change-notices/read", 41, `not json`), permissions.UsersRead)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	serveRoute(t, &careplanhttp.Resource{}, w, pickupNoticeRequest(http.MethodGet, "/students/pickup-change-notices", 41, ""), permissions.UsersRead)
	assert.Equal(t, http.StatusInternalServerError, w.Code, "an unbound capability is a server error, never an empty list")
}

func mustData(t *testing.T, w *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Data
}
