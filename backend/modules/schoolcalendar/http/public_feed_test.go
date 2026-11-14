package staffcalendarhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	calendarSvc "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal"
	"github.com/stretchr/testify/assert"
)

type publicFeeds struct {
	parent, staff map[string]string
	fail          error
}

// CalendarFeedByToken mirrors the service's order: parent feed first, the
// staff feed only when the parent lookup found nothing.
func (f publicFeeds) CalendarFeedByToken(_ context.Context, token string) (string, string, error) {
	filename, content, err := feedByToken(f.parent, token, f.fail)
	if errors.Is(err, calendarSvc.ErrNotFound) {
		return feedByToken(f.staff, token, nil)
	}
	return filename, content, err
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

// A feed read the calendar app canceled is no server error: 499.
func TestPublicFeedAnswersAClientCancellationWith499(t *testing.T) {
	t.Parallel()
	router := chi.NewRouter()
	router.Get("/public/calendar/{token}", PublicFeed(publicFeeds{fail: context.Canceled}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/public/calendar/p", nil).WithContext(ctx))
	assert.Equal(t, 499, recorder.Code, recorder.Body.String())
}
