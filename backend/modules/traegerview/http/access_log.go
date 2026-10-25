package traegerviewhttp

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AccessLogResource serves the access log of the Träger-Portal (#4655):
// who read or changed personal data at a school of the Träger, and when.
// Every route needs the right „Zugriffsprotokoll lesen“.
type AccessLogResource struct {
	overview traegerview.Overview
}

// NewAccessLogResource builds the resource.
func NewAccessLogResource(overview traegerview.Overview) *AccessLogResource {
	if overview == nil {
		panic("traeger view http: the overview is required")
	}
	return &AccessLogResource{overview: overview}
}

// TraegerRouter returns the routes mounted at /traeger/access-log inside the
// Träger route group the portal supplies.
func (rs *AccessLogResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router) {
		r.Use(common.RequiresPermission(identityaccess.TraegerRightAccessLogRead))
		r.Get("/", rs.list)
		r.Get("/people", rs.people)
		r.Get("/export", rs.export)
	})
	return r
}

// list answers GET /traeger/access-log: one page of the filter.
func (rs *AccessLogResource) list(w http.ResponseWriter, r *http.Request) {
	viewer, ok := traegerViewer(w, r)
	if !ok {
		return
	}
	filter, err := accessLogFilter(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		if page, err = strconv.Atoi(raw); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("%w: page %q", traegerview.ErrInvalidAccessLogFilter, raw)))
			return
		}
	}
	result, err := rs.overview.AccessLog(r.Context(), viewer, filter, page)
	if err != nil {
		renderAccessLogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Access log retrieved successfully")
}

// people answers GET /traeger/access-log/people: the people of the window,
// for the person filter.
func (rs *AccessLogResource) people(w http.ResponseWriter, r *http.Request) {
	viewer, ok := traegerViewer(w, r)
	if !ok {
		return
	}
	from, to, err := accessLogDays(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	people, err := rs.overview.AccessLogPeople(r.Context(), viewer, from, to)
	if err != nil {
		renderAccessLogError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, people, "Access log people retrieved successfully")
}

// export answers GET /traeger/access-log/export: every entry of the filter
// as CSV.
func (rs *AccessLogResource) export(w http.ResponseWriter, r *http.Request) {
	viewer, ok := traegerViewer(w, r)
	if !ok {
		return
	}
	filter, err := accessLogFilter(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	entries, err := rs.overview.AccessLogExport(r.Context(), viewer, filter)
	if err != nil {
		renderAccessLogError(w, r, err)
		return
	}
	data, err := accessLogCSV(entries)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="zugriffsprotokoll-%s-bis-%s.csv"`, filter.From, filter.To))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func traegerViewer(w http.ResponseWriter, r *http.Request) (traegerview.Viewer, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return traegerview.Viewer{}, false
	}
	return traegerview.Viewer{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

func renderAccessLogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, traegerview.ErrInvalidAccessLogFilter):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, traegerview.ErrAccessLogExportTooLarge):
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeTraegerAccessLogExportTooLarge,
			map[string]any{"limit": traegerview.AccessLogExportLimit}))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// accessLogDays reads the window: from and to as YYYY-MM-DD, both required.
func accessLogDays(r *http.Request) (calendar.Date, calendar.Date, error) {
	query := r.URL.Query()
	from, err := calendar.ParseDate(query.Get("from"))
	if err != nil {
		return "", "", fmt.Errorf("%w: from must be a date (YYYY-MM-DD)", traegerview.ErrInvalidAccessLogFilter)
	}
	to, err := calendar.ParseDate(query.Get("to"))
	if err != nil {
		return "", "", fmt.Errorf("%w: to must be a date (YYYY-MM-DD)", traegerview.ErrInvalidAccessLogFilter)
	}
	return from, to, nil
}

// accessLogFilter reads the window and the optional school_id, account_id
// and kind.
func accessLogFilter(r *http.Request) (traegerview.AccessLogFilter, error) {
	from, to, err := accessLogDays(r)
	if err != nil {
		return traegerview.AccessLogFilter{}, err
	}
	filter := traegerview.AccessLogFilter{From: from, To: to, Kind: r.URL.Query().Get("kind")}
	if raw := r.URL.Query().Get("as_of"); raw != "" {
		if filter.AsOf, err = time.Parse(time.RFC3339Nano, raw); err != nil {
			return traegerview.AccessLogFilter{}, fmt.Errorf("%w: as_of must be an RFC 3339 instant", traegerview.ErrInvalidAccessLogFilter)
		}
	}
	if filter.SchoolID, err = optionalID(r, "school_id"); err != nil {
		return traegerview.AccessLogFilter{}, err
	}
	if filter.AccountID, err = optionalID(r, "account_id"); err != nil {
		return traegerview.AccessLogFilter{}, err
	}
	return filter, nil
}

func optionalID(r *http.Request, name string) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: %s must be a positive ID", traegerview.ErrInvalidAccessLogFilter, name)
	}
	return id, nil
}

// berlin is the time zone the export states times in.
var berlin = func() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.FixedZone("CET", 60*60)
	}
	return location
}()

var accessKindLabels = map[string]string{
	traegerview.AccessKindRead:   "Gelesen",
	traegerview.AccessKindChange: "Geändert",
}

// accessLogCSV is the export: semicolon separated, UTF-8 with BOM so German
// spreadsheet programs open it correctly.
func accessLogCSV(entries []traegerview.AccessLogEntry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'
	rows := [][]string{{"Datum", "Uhrzeit", "Schule", "Person", "Art", "Daten"}}
	for _, entry := range entries {
		at := entry.OccurredAt.In(berlin)
		rows = append(rows, []string{
			at.Format("02.01.2006"), at.Format("15:04:05"), csvSafe(entry.SchoolName), csvSafe(entry.Person),
			accessKindLabels[entry.Kind], entry.DataLabel,
		})
	}
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// csvSafe keeps a stored name from being read as a spreadsheet formula.
func csvSafe(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}
