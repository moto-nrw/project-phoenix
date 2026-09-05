package calendar

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
)

const (
	calDAVRoot          = "/api/caldav/"
	calDAVPrincipalPath = "/api/caldav/principal/"
	calDAVCalendarPath  = "/api/caldav/calendars/personal/"
	calDAVRealm         = `Basic realm="moto Kalender", charset="UTF-8"`
	calDAVContentType   = "application/xml; charset=utf-8"
)

func mustNewStaffCalDAVHandler(service calendarService.StaffCalDAVService, logger *slog.Logger) http.Handler {
	if service == nil {
		panic("configure staff CalDAV handler: service is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &staffCalDAVHandler{service: service, logger: logger}
}

type staffCalDAVHandler struct {
	service calendarService.StaffCalDAVService
	logger  *slog.Logger
}

func (h *staffCalDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	username, password, ok := r.BasicAuth()
	if !ok {
		h.unauthorized(w)
		return
	}
	snapshot, err := h.service.AuthenticateStaffCalDAV(r.Context(), username, password)
	if err != nil {
		if !errors.Is(err, calendarService.ErrNotFound) && !errors.Is(err, calendarService.ErrForbidden) {
			h.logger.ErrorContext(r.Context(), "authenticate caldav request", "error", err)
		}
		h.unauthorized(w)
		return
	}
	path := r.URL.EscapedPath()
	if path == "/.well-known/caldav" || path == strings.TrimSuffix(calDAVRoot, "/") || path == calDAVRoot {
		http.Redirect(w, r, calDAVPrincipalPath, http.StatusMovedPermanently)
		return
	}

	switch r.Method {
	case http.MethodOptions:
		h.options(w, r, path)
	case "PROPFIND":
		h.propfind(w, r, path, snapshot)
	case "REPORT":
		h.report(w, r, path, snapshot)
	case http.MethodGet, http.MethodHead:
		h.getItem(w, r, path, snapshot)
	default:
		setCalDAVAllow(w, path)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (h *staffCalDAVHandler) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", calDAVRealm)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

func (*staffCalDAVHandler) options(w http.ResponseWriter, r *http.Request, path string) {
	if !knownCalDAVPath(path) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("DAV", "1, 2, access-control, calendar-access")
	setCalDAVAllow(w, path)
	w.WriteHeader(http.StatusNoContent)
}

func setCalDAVAllow(w http.ResponseWriter, path string) {
	methods := "OPTIONS, PROPFIND, REPORT, GET, HEAD"
	switch path {
	case calDAVPrincipalPath, strings.TrimSuffix(calDAVPrincipalPath, "/"):
		methods = "OPTIONS, PROPFIND"
	case calDAVCalendarPath, strings.TrimSuffix(calDAVCalendarPath, "/"):
		methods = "OPTIONS, PROPFIND, REPORT"
	default:
		if strings.HasPrefix(path, calDAVCalendarPath) {
			methods = "OPTIONS, PROPFIND, GET, HEAD"
		}
	}
	w.Header().Set("Allow", methods)
}

func (h *staffCalDAVHandler) propfind(w http.ResponseWriter, r *http.Request, path string, snapshot *calendarService.StaffCalDAVCalendar) {
	depth := r.Header.Get("Depth")
	if depth != "" && depth != "0" && depth != "1" {
		http.Error(w, "Only Depth 0 and 1 are supported", http.StatusBadRequest)
		return
	}

	var responses []string
	switch {
	case path == calDAVPrincipalPath || path == strings.TrimSuffix(calDAVPrincipalPath, "/"):
		responses = append(responses, calDAVPrincipalResponse())
		if depth == "1" {
			responses = append(responses, calDAVCalendarResponse(snapshot))
		}
	case path == calDAVCalendarPath || path == strings.TrimSuffix(calDAVCalendarPath, "/"):
		responses = append(responses, calDAVCalendarResponse(snapshot))
		if depth == "1" {
			for _, item := range snapshot.Items {
				responses = append(responses, calDAVItemResponse(calDAVItemPath(item), item, false))
			}
		}
	default:
		item, ok := findCalDAVItem(path, snapshot)
		if !ok {
			http.NotFound(w, r)
			return
		}
		responses = append(responses, calDAVItemResponse(calDAVItemPath(item), item, true))
	}
	h.writeMultiStatus(w, responses)
}

func (h *staffCalDAVHandler) report(w http.ResponseWriter, r *http.Request, path string, snapshot *calendarService.StaffCalDAVCalendar) {
	if path != calDAVCalendarPath && path != strings.TrimSuffix(calDAVCalendarPath, "/") {
		http.NotFound(w, r)
		return
	}
	report, hrefs, err := parseCalDAVReport(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var responses []string
	switch report {
	case "calendar-query":
		for _, item := range snapshot.Items {
			responses = append(responses, calDAVItemResponse(calDAVItemPath(item), item, true))
		}
	case "calendar-multiget":
		for _, href := range hrefs {
			item, ok := findCalDAVItem(href, snapshot)
			if !ok {
				responses = append(responses, calDAVNotFoundResponse(href))
				continue
			}
			responses = append(responses, calDAVItemResponse(calDAVItemPath(item), item, true))
		}
	default:
		http.Error(w, "Unsupported CalDAV report", http.StatusForbidden)
		return
	}
	h.writeMultiStatus(w, responses)
}

func (*staffCalDAVHandler) getItem(w http.ResponseWriter, r *http.Request, path string, snapshot *calendarService.StaffCalDAVCalendar) {
	item, ok := findCalDAVItem(path, snapshot)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("ETag", item.ETag)
	if !item.ModifiedAt.IsZero() {
		w.Header().Set("Last-Modified", item.ModifiedAt.UTC().Format(http.TimeFormat))
	}
	if etagMatches(r.Header.Get("If-None-Match"), item.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(item.Content)
	}
}

func (*staffCalDAVHandler) writeMultiStatus(w http.ResponseWriter, responses []string) {
	w.Header().Set("Content-Type", calDAVContentType)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
		`<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/">`+
		strings.Join(responses, "")+`</d:multistatus>`)
}

func calDAVPrincipalResponse() string {
	return `<d:response><d:href>` + xmlText(calDAVPrincipalPath) + `</d:href>` +
		`<d:propstat><d:prop>` +
		`<d:displayname>moto Kalender</d:displayname>` +
		`<d:resourcetype><d:collection/><d:principal/></d:resourcetype>` +
		`<d:current-user-principal><d:href>` + xmlText(calDAVPrincipalPath) + `</d:href></d:current-user-principal>` +
		`<d:principal-URL><d:href>` + xmlText(calDAVPrincipalPath) + `</d:href></d:principal-URL>` +
		`<c:calendar-home-set><d:href>` + xmlText(calDAVCalendarPath) + `</d:href></c:calendar-home-set>` +
		calDAVSupportedReports() +
		`</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
}

func calDAVCalendarResponse(snapshot *calendarService.StaffCalDAVCalendar) string {
	return `<d:response><d:href>` + xmlText(calDAVCalendarPath) + `</d:href>` +
		`<d:propstat><d:prop>` +
		`<d:displayname>moto Termine</d:displayname>` +
		`<c:calendar-description>Persönlicher Kalender aus moto</c:calendar-description>` +
		`<d:resourcetype><d:collection/><c:calendar/></d:resourcetype>` +
		`<d:owner><d:href>` + xmlText(calDAVPrincipalPath) + `</d:href></d:owner>` +
		`<d:current-user-privilege-set><d:privilege><d:read/></d:privilege><d:privilege><d:read-current-user-privilege-set/></d:privilege></d:current-user-privilege-set>` +
		`<c:supported-calendar-component-set><c:comp name="VEVENT"/></c:supported-calendar-component-set>` +
		`<cs:getctag>` + xmlText(snapshot.Revision) + `</cs:getctag>` +
		calDAVSupportedReports() +
		`</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
}

func calDAVSupportedReports() string {
	return `<d:supported-report-set>` +
		`<d:supported-report><d:report><c:calendar-query/></d:report></d:supported-report>` +
		`<d:supported-report><d:report><c:calendar-multiget/></d:report></d:supported-report>` +
		`</d:supported-report-set>`
}

func calDAVItemResponse(href string, item calendarService.StaffCalDAVItem, includeData bool) string {
	data := ""
	if includeData {
		data = `<c:calendar-data content-type="text/calendar" version="2.0">` + xmlText(string(item.Content)) + `</c:calendar-data>`
	}
	return `<d:response><d:href>` + xmlText(href) + `</d:href>` +
		`<d:propstat><d:prop><d:getetag>` + xmlText(item.ETag) + `</d:getetag>` +
		`<d:getcontenttype>text/calendar; charset=utf-8</d:getcontenttype><d:resourcetype/>` + data +
		`</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
}

func calDAVNotFoundResponse(href string) string {
	return `<d:response><d:href>` + xmlText(href) + `</d:href><d:status>HTTP/1.1 404 Not Found</d:status></d:response>`
}

func calDAVItemPath(item calendarService.StaffCalDAVItem) string {
	return calDAVCalendarPath + url.PathEscape(item.Name)
}

func findCalDAVItem(rawPath string, snapshot *calendarService.StaffCalDAVCalendar) (calendarService.StaffCalDAVItem, bool) {
	parsed, err := url.Parse(rawPath)
	if err != nil {
		return calendarService.StaffCalDAVItem{}, false
	}
	path := parsed.EscapedPath()
	name, ok := strings.CutPrefix(path, calDAVCalendarPath)
	if !ok || name == "" || strings.Contains(name, "/") {
		return calendarService.StaffCalDAVItem{}, false
	}
	name, err = url.PathUnescape(name)
	if err != nil {
		return calendarService.StaffCalDAVItem{}, false
	}
	for _, item := range snapshot.Items {
		if item.Name == name {
			return item, true
		}
	}
	return calendarService.StaffCalDAVItem{}, false
}

func knownCalDAVPath(path string) bool {
	return path == calDAVPrincipalPath || path == strings.TrimSuffix(calDAVPrincipalPath, "/") ||
		path == calDAVCalendarPath || path == strings.TrimSuffix(calDAVCalendarPath, "/") ||
		strings.HasPrefix(path, calDAVCalendarPath)
}

func parseCalDAVReport(body io.Reader) (string, []string, error) {
	decoder := xml.NewDecoder(io.LimitReader(body, 1<<20))
	var root string
	var hrefs []string
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		root, err = calDAVReportRoot(root, start)
		if err != nil {
			return "", nil, err
		}
		href, err := decodeCalDAVHref(decoder, start)
		if err != nil {
			return "", nil, err
		}
		if href != "" {
			hrefs = append(hrefs, href)
		}
	}
	if root == "" {
		return "", nil, errors.New("empty CalDAV report")
	}
	return root, hrefs, nil
}

func calDAVReportRoot(current string, start xml.StartElement) (string, error) {
	if current != "" {
		return current, nil
	}
	if start.Name.Space != "urn:ietf:params:xml:ns:caldav" {
		return "", errors.New("unsupported CalDAV report namespace")
	}
	return start.Name.Local, nil
}

func decodeCalDAVHref(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	if start.Name.Local != "href" {
		return "", nil
	}
	var href string
	if err := decoder.DecodeElement(&href, &start); err != nil {
		return "", err
	}
	return strings.TrimSpace(href), nil
}

func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimSpace(candidate) == etag || strings.TrimSpace(candidate) == "*" {
			return true
		}
	}
	return false
}

func xmlText(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

func (rs *Resource) ServeCalDAV(w http.ResponseWriter, r *http.Request) {
	rs.calDAVHandler.ServeHTTP(w, r)
}
