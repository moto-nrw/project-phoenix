package calendar

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/stretchr/testify/assert"
)

const testCalDAVEvent = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//moto//Kalender//DE\r\n" +
	"CALSCALE:GREGORIAN\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:shift-1@moto-app.de\r\n" +
	"DTSTAMP:20260905T100000Z\r\n" +
	"DTSTART;VALUE=DATE:20260907\r\n" +
	"DTEND;VALUE=DATE:20260908\r\n" +
	"SUMMARY:Dienst\r\n" +
	"STATUS:CONFIRMED\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func calDAVTestResource() (*Resource, *fakeCalendarService) {
	service := &fakeCalendarService{
		calDAVCalendar: &calendarSvc.StaffCalDAVCalendar{
			AccountID: "staff:12:34",
			TenantID:  12,
			Revision:  "calendar-revision",
			Items: []calendarSvc.StaffCalDAVItem{{
				Name:       "event.ics",
				UID:        "shift-1@moto-app.de",
				Content:    []byte(testCalDAVEvent),
				ETag:       `"event-etag"`,
				ModifiedAt: time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC),
			}},
		},
	}
	return NewResource(service, nil, slog.Default()), service
}

func performCalDAVRequest(t *testing.T, resource *Resource, method, target, body string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	}
	if authenticated {
		req.SetBasicAuth("staff@example.org", "app-password")
	}
	res := httptest.NewRecorder()
	resource.ServeCalDAV(res, req)
	return res
}

func TestStaffCalDAVRequiresUniformBasicAuthentication(t *testing.T) {
	t.Parallel()
	resource, service := calDAVTestResource()

	for _, path := range []string{"/.well-known/caldav", calDAVPrincipalPath, calDAVCalendarPath, calDAVCalendarPath + "event.ics"} {
		missing := performCalDAVRequest(t, resource, "PROPFIND", path, "", false)
		assert.Equal(t, http.StatusUnauthorized, missing.Code)
		assert.Equal(t, calDAVRealm, missing.Header().Get("WWW-Authenticate"))
	}

	service.calDAVErr = calendarSvc.ErrNotFound
	wrong := performCalDAVRequest(t, resource, "PROPFIND", calDAVPrincipalPath, "", true)
	assert.Equal(t, http.StatusUnauthorized, wrong.Code)
	assert.Equal(t, "Unauthorized\n", wrong.Body.String())
	assert.Equal(t, "staff@example.org", service.calDAVUsername)
	assert.Equal(t, "app-password", service.calDAVPassword)
}

func TestStaffCalDAVDiscoveryAndReadOnlyMethods(t *testing.T) {
	t.Parallel()
	resource, _ := calDAVTestResource()

	discovery := performCalDAVRequest(t, resource, http.MethodGet, "/.well-known/caldav", "", true)
	assert.Equal(t, http.StatusMovedPermanently, discovery.Code)
	assert.Equal(t, calDAVPrincipalPath, discovery.Header().Get("Location"))

	root := performCalDAVRequest(t, resource, "PROPFIND", calDAVPrincipalPath, "", true)
	assert.Equal(t, http.StatusMultiStatus, root.Code)
	assert.Contains(t, root.Body.String(), "calendar-home-set")
	assert.Contains(t, root.Body.String(), `<c:calendar-home-set><d:href>`+calDAVCalendarPath+`</d:href></c:calendar-home-set>`)

	principalDepthOneRequest := httptest.NewRequest("PROPFIND", calDAVPrincipalPath, nil)
	principalDepthOneRequest.SetBasicAuth("staff@example.org", "app-password")
	principalDepthOneRequest.Header.Set("Depth", "1")
	principalDepthOne := httptest.NewRecorder()
	resource.ServeCalDAV(principalDepthOne, principalDepthOneRequest)
	assert.Equal(t, http.StatusMultiStatus, principalDepthOne.Code)
	assert.Contains(t, principalDepthOne.Body.String(), calDAVCalendarPath)

	calendar := performCalDAVRequest(t, resource, "PROPFIND", calDAVCalendarPath, "", true)
	assert.Equal(t, http.StatusMultiStatus, calendar.Code)
	assert.Contains(t, calendar.Body.String(), "moto Termine")
	assert.Contains(t, calendar.Body.String(), "VEVENT")

	options := performCalDAVRequest(t, resource, http.MethodOptions, calDAVCalendarPath, "", true)
	assert.Equal(t, http.StatusNoContent, options.Code)
	assert.ElementsMatch(t, []string{"OPTIONS", "PROPFIND", "REPORT"}, splitAllow(options.Header().Get("Allow")))
}

func TestStaffCalDAVGetHeadQueryAndMultiget(t *testing.T) {
	t.Parallel()
	resource, _ := calDAVTestResource()
	itemPath := calDAVCalendarPath + "event.ics"

	get := performCalDAVRequest(t, resource, http.MethodGet, itemPath, "", true)
	assert.Equal(t, http.StatusOK, get.Code)
	assert.Equal(t, testCalDAVEvent, get.Body.String())
	assert.Equal(t, "text/calendar; charset=utf-8", get.Header().Get("Content-Type"))
	assert.NotEmpty(t, get.Header().Get("ETag"))
	assert.False(t, strings.HasPrefix(get.Header().Get("ETag"), "W/"))
	assert.Equal(t, "Sat, 05 Sep 2026 10:00:00 GMT", get.Header().Get("Last-Modified"))

	head := performCalDAVRequest(t, resource, http.MethodHead, itemPath, "", true)
	assert.Equal(t, http.StatusOK, head.Code)
	assert.Empty(t, head.Body.String())
	assert.Equal(t, get.Header().Get("ETag"), head.Header().Get("ETag"))

	conditionalRequest := httptest.NewRequest(http.MethodGet, itemPath, nil)
	conditionalRequest.SetBasicAuth("staff@example.org", "app-password")
	conditionalRequest.Header.Set("If-None-Match", get.Header().Get("ETag"))
	conditional := httptest.NewRecorder()
	resource.ServeCalDAV(conditional, conditionalRequest)
	assert.Equal(t, http.StatusNotModified, conditional.Code)
	assert.Empty(t, conditional.Body.String())

	queryBody := `<?xml version="1.0" encoding="utf-8" ?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop><d:getetag/><c:calendar-data/></d:prop>
  <c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"><c:time-range start="20260901T000000Z" end="20261001T000000Z"/></c:comp-filter></c:comp-filter></c:filter>
</c:calendar-query>`
	query := performCalDAVRequest(t, resource, "REPORT", calDAVCalendarPath, queryBody, true)
	assert.Equal(t, http.StatusMultiStatus, query.Code)
	assert.Contains(t, query.Body.String(), "event.ics")
	assert.Contains(t, query.Body.String(), "shift-1@moto-app.de")

	multigetBody := `<?xml version="1.0" encoding="utf-8" ?>
<c:calendar-multiget xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop><d:getetag/><c:calendar-data/></d:prop>
  <d:href>` + itemPath + `</d:href>
  <d:href>` + calDAVCalendarPath + `unknown.ics</d:href>
</c:calendar-multiget>`
	multiget := performCalDAVRequest(t, resource, "REPORT", calDAVCalendarPath, multigetBody, true)
	assert.Equal(t, http.StatusMultiStatus, multiget.Code)
	assert.Contains(t, multiget.Body.String(), "event.ics")
	assert.Contains(t, multiget.Body.String(), "unknown.ics")
	assert.Contains(t, multiget.Body.String(), "404")
}

func TestStaffCalDAVETagChangesWithContentButKeepsTheObjectURL(t *testing.T) {
	t.Parallel()
	resource, service := calDAVTestResource()
	itemPath := calDAVCalendarPath + "event.ics"

	before := performCalDAVRequest(t, resource, http.MethodGet, itemPath, "", true)
	assert.Equal(t, http.StatusOK, before.Code)

	service.calDAVCalendar.Items[0].Content = []byte(strings.Replace(
		testCalDAVEvent,
		"SUMMARY:Dienst",
		"SUMMARY:Geänderter Dienst",
		1,
	))
	service.calDAVCalendar.Items[0].ETag = `"changed-event-etag"`
	after := performCalDAVRequest(t, resource, http.MethodGet, itemPath, "", true)
	assert.Equal(t, http.StatusOK, after.Code)
	assert.NotEqual(t, before.Header().Get("ETag"), after.Header().Get("ETag"))
	assert.Contains(t, after.Body.String(), "SUMMARY:Geänderter Dienst")
}

func TestStaffCalDAVRejectsAllWrites(t *testing.T) {
	t.Parallel()
	resource, _ := calDAVTestResource()
	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCALENDAR", "MOVE", "COPY"} {
		t.Run(method, func(t *testing.T) {
			target := calDAVCalendarPath + "event.ics"
			if method == "MKCALENDAR" {
				target = "/api/caldav/calendars/other/"
			}
			response := performCalDAVRequest(t, resource, method, target, testCalDAVEvent, true)
			assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
		})
	}
}

func splitAllow(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
