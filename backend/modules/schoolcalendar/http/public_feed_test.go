package staffcalendarhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/stretchr/testify/assert"
)

type publicFeeds struct {
	parent, staff map[string]string
	fail          error
}

func (f publicFeeds) ParentCalendarFeedByToken(_ context.Context, token string) (string, string, error) {
	return feedByToken(f.parent, token, f.fail)
}

func (f publicFeeds) StaffCalendarFeedByToken(_ context.Context, token string) (string, string, error) {
	return feedByToken(f.staff, token, nil)
}

func feedByToken(feeds map[string]string, token string, fail error) (string, string, error) {
	if fail != nil {
		return "", "", fail
	}
	if content, ok := feeds[token]; ok {
		return token + ".ics", content, nil
	}
	return "", "", calendarSvc.ErrNotFound
}

func servePublicFeed(feeds PublicFeeds, token string) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Get("/public/calendar/{token}", PublicFeed(feeds))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/public/calendar/"+token, nil))
	return recorder
}

func TestPublicFeedTriesTheParentFeedThenTheStaffFeed(t *testing.T) {
	t.Parallel()
	feeds := publicFeeds{parent: map[string]string{"p": "PARENT"}, staff: map[string]string{"s": "STAFF", "p": "SHADOWED"}}

	parent := servePublicFeed(feeds, "p")
	assert.Equal(t, http.StatusOK, parent.Code)
	assert.Equal(t, "PARENT", parent.Body.String())
	assert.Equal(t, "text/calendar; charset=utf-8", parent.Header().Get("Content-Type"))
	assert.Equal(t, `inline; filename="p.ics"`, parent.Header().Get("Content-Disposition"))
	assert.Equal(t, "private, max-age=3600", parent.Header().Get("Cache-Control"))

	staff := servePublicFeed(feeds, "s")
	assert.Equal(t, http.StatusOK, staff.Code)
	assert.Equal(t, "STAFF", staff.Body.String())

	assert.Equal(t, http.StatusNotFound, servePublicFeed(feeds, "unknown").Code)
}

func TestPublicFeedReportsFailuresAndAMissingCalendar(t *testing.T) {
	t.Parallel()
	assert.Equal(t, http.StatusInternalServerError, servePublicFeed(publicFeeds{fail: errors.New("down")}, "p").Code)
	assert.Equal(t, http.StatusNotFound, servePublicFeed(nil, "p").Code)
}
