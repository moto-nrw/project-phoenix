package careplanhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	careplan "github.com/moto-ogs/project-phoenix/modules/careplan"
	sessiontoken "github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

func pickupNoticeRequest(method, target string, accountID int, id, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: accountID})
	if id != "" {
		route := chi.NewRouteContext()
		route.URLParams.Add("id", id)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
	}
	return req.WithContext(ctx)
}

// The header reads only the caller's own markers, with the child's first
// name and the day as YYYY-MM-DD; opening a child clears that child's.
func TestPickupChangeNoticeRoutesServeTheCallersMarkers(t *testing.T) {
	t.Parallel()
	informed := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	notices := &pickupNoticesDouble{notices: []careplan.PickupChangeTeamNotice{{
		RequestID: 5, StudentID: 77, StudentFirstName: "Mia", Date: calendar.NewDate(2026, 11, 13), PickupTime: "15:00", InformedAt: informed,
	}}}
	rs := &Resource{ResourceConfig: ResourceConfig{PickupChangeNotices: notices}}

	w := httptest.NewRecorder()
	rs.listPickupChangeNotices(w, pickupNoticeRequest(http.MethodGet, "/pickup-change-notices", 41, "", ""))
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
	rs.readPickupChangeNotices(w, pickupNoticeRequest(http.MethodPost, "/77/pickup-change-notices/read", 41, "77",
		`{"notices":[{"request_id":"5","informed_at":"2026-11-12T08:00:00.123456Z"}]}`))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(77), notices.student)
	assert.Equal(t, []careplan.PickupChangeNoticeRevision{{RequestID: 5, InformedAt: time.Date(2026, 11, 12, 8, 0, 0, 123456000, time.UTC)}}, notices.observed)
	assert.JSONEq(t, `[]`, string(mustData(t, w)))

	w = httptest.NewRecorder()
	rs.readPickupChangeNotices(w, pickupNoticeRequest(http.MethodPost, "/x/pickup-change-notices/read", 41, "x", `{"notices":[]}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	rs.readPickupChangeNotices(w, pickupNoticeRequest(http.MethodPost, "/77/pickup-change-notices/read", 41, "77", `not json`))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	(&Resource{}).listPickupChangeNotices(w, pickupNoticeRequest(http.MethodGet, "/pickup-change-notices", 41, "", ""))
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
