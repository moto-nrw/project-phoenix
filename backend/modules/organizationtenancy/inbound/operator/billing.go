package operator

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// BillingResource serves the operator billing report (#2791): the key day
// that applies to every school and the monthly key-date counts, as JSON and
// as a CSV download.
type BillingResource struct {
	service            organizationtenancy.BillingReport
	logger             *slog.Logger
	isLocalSeedRequest func(*http.Request) bool
}

// NewBillingResource creates the billing routes' handlers.
// A nil logger falls back to slog.Default.
func NewBillingResource(service organizationtenancy.BillingReport, logger *slog.Logger, isLocalSeedRequest func(*http.Request) bool) *BillingResource {
	if service == nil {
		panic("organization tenancy operator: billing report is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingResource{service: service, logger: logger, isLocalSeedRequest: isLocalSeedRequest}
}

type billingKeyDayResponse struct {
	KeyDay      int       `json:"key_day"`
	NextKeyDate string    `json:"next_key_date"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type billingKeyDateCountResponse struct {
	SchoolID         string    `json:"school_id"`
	SchoolName       string    `json:"school_name"`
	OrganizationName string    `json:"organization_name"`
	Period           string    `json:"period"`
	KeyDate          string    `json:"key_date"`
	ActiveStudents   int       `json:"active_students"`
	ActiveTerminals  int       `json:"active_terminals"`
	RecordedAt       time.Time `json:"recorded_at"`
}

type updateBillingKeyDayRequest struct {
	KeyDay *int `json:"key_day"`
}

func (req *updateBillingKeyDayRequest) Bind(_ *http.Request) error {
	if req.KeyDay == nil {
		return errors.New("key_day is required")
	}
	return nil
}

func toBillingKeyDayResponse(keyDay organizationtenancy.BillingKeyDay) billingKeyDayResponse {
	return billingKeyDayResponse{KeyDay: keyDay.Day, NextKeyDate: keyDay.NextKeyDate, UpdatedAt: keyDay.UpdatedAt}
}

func toBillingKeyDateCountResponse(count organizationtenancy.BillingKeyDateCount) billingKeyDateCountResponse {
	return billingKeyDateCountResponse{
		SchoolID:         strconv.FormatInt(count.SchoolID, 10),
		SchoolName:       count.SchoolName,
		OrganizationName: count.OrganizationName,
		Period:           count.Period,
		KeyDate:          count.KeyDate,
		ActiveStudents:   count.ActiveStudents,
		ActiveTerminals:  count.ActiveTerminals,
		RecordedAt:       count.RecordedAt,
	}
}

// GetKeyDay returns the billing key day. GET /operator/billing/key-day.
func (rs *BillingResource) GetKeyDay(w http.ResponseWriter, r *http.Request) {
	keyDay, err := rs.service.BillingKeyDay(r.Context())
	if err != nil {
		common.RenderError(w, r, rs.billingError(r, err))
		return
	}
	common.Respond(w, r, http.StatusOK, toBillingKeyDayResponse(keyDay), "Billing key day retrieved successfully")
}

// UpdateKeyDay changes the billing key day. PUT /operator/billing/key-day.
func (rs *BillingResource) UpdateKeyDay(w http.ResponseWriter, r *http.Request) {
	req := &updateBillingKeyDayRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	keyDay, err := rs.service.SetBillingKeyDay(r.Context(), *req.KeyDay, operatorID)
	if err != nil {
		common.RenderError(w, r, rs.billingError(r, err))
		return
	}
	common.Respond(w, r, http.StatusOK, toBillingKeyDayResponse(keyDay), "Billing key day updated successfully")
}

// ListKeyDateCounts returns every captured month. GET
// /operator/billing/key-date-counts.
func (rs *BillingResource) ListKeyDateCounts(w http.ResponseWriter, r *http.Request) {
	counts, err := rs.service.ListBillingKeyDateCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, rs.billingError(r, err))
		return
	}
	result := make([]billingKeyDateCountResponse, 0, len(counts))
	for _, count := range counts {
		result = append(result, toBillingKeyDateCountResponse(count))
	}
	common.Respond(w, r, http.StatusOK, result, "Billing key-date counts retrieved successfully")
}

// SeedKeyDateCounts writes the deterministic demo snapshot. It is only
// reachable by the local API seeder and never enables a production capture.
func (rs *BillingResource) SeedKeyDateCounts(w http.ResponseWriter, r *http.Request) {
	if rs.isLocalSeedRequest == nil || !rs.isLocalSeedRequest(r) {
		common.RenderError(w, r, common.OperatorForbidden("Die Demo-Erfassung ist nur lokal verfügbar."))
		return
	}
	written, err := rs.service.SeedBillingKeyDateCounts(r.Context(), time.Now())
	if err != nil {
		common.RenderError(w, r, rs.billingError(r, err))
		return
	}
	common.Respond(w, r, http.StatusCreated, map[string]int{"written": written}, "Demo-Stichtagszahlen erfasst")
}

var billingMonthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// ExportKeyDateCounts downloads the captured months as CSV. The optional
// month query parameter (YYYY-MM) limits it to one month. GET
// /operator/billing/key-date-counts/export.
func (rs *BillingResource) ExportKeyDateCounts(w http.ResponseWriter, r *http.Request) {
	month := strings.TrimSpace(r.URL.Query().Get("month"))
	if month != "" && !billingMonthPattern.MatchString(month) {
		common.RenderError(w, r, common.OperatorInvalidRequest(errors.New("month must be YYYY-MM")))
		return
	}
	counts, err := rs.service.ListBillingKeyDateCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, rs.billingError(r, err))
		return
	}
	if month != "" {
		counts = billingCountsOfMonth(counts, month+"-01")
	}
	body, err := billingCSV(counts)
	if err != nil {
		common.RenderError(w, r, rs.billingError(r, err))
		return
	}
	filename := "stichtagszahlen.csv"
	if month != "" {
		filename = "stichtagszahlen-" + month + ".csv"
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func billingCountsOfMonth(counts []organizationtenancy.BillingKeyDateCount, period string) []organizationtenancy.BillingKeyDateCount {
	result := make([]organizationtenancy.BillingKeyDateCount, 0, len(counts))
	for _, count := range counts {
		if count.Period == period {
			result = append(result, count)
		}
	}
	return result
}

// billingCSV writes the rows for a spreadsheet: UTF-8 with BOM so Excel reads
// the umlauts, semicolons as German Excel expects, German dates, and every
// text cell guarded against formula injection.
func billingCSV(counts []organizationtenancy.BillingKeyDateCount) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'
	header := []string{"Monat", "Stichtag", "Träger", "Schule", "Schul-ID", "Aktive Kinder", "Aktive Terminals", "Erfasst am"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	for _, count := range counts {
		record := []string{
			germanDate(count.Period, "01/2006"),
			germanDate(count.KeyDate, "02.01.2006"),
			sanitizeCSVCell(count.OrganizationName),
			sanitizeCSVCell(count.SchoolName),
			strconv.FormatInt(count.SchoolID, 10),
			strconv.Itoa(count.ActiveStudents),
			strconv.Itoa(count.ActiveTerminals),
			count.RecordedAt.Format("02.01.2006 15:04"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("write billing csv: %w", err)
	}
	return buffer.Bytes(), nil
}

// germanDate renders a YYYY-MM-DD calendar day in layout; a value that is
// not such a day stays as it is.
func germanDate(day, layout string) string {
	if len(day) != len("2006-01-02") || day[4] != '-' || day[7] != '-' {
		return day
	}
	switch layout {
	case "01/2006":
		return day[5:7] + "/" + day[:4]
	case "02.01.2006":
		return day[8:] + "." + day[5:7] + "." + day[:4]
	default:
		return day
	}
}

// sanitizeCSVCell prefixes a cell a spreadsheet would read as a formula.
func sanitizeCSVCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}
	return value
}

// billingError answers a refused key day with 400 and logs every other
// failure before it answers 500 without details.
func (rs *BillingResource) billingError(r *http.Request, err error) render.Renderer {
	if errors.Is(err, organizationtenancy.ErrInvalidBillingKeyDay) {
		return common.OperatorInvalidRequest(err)
	}
	rs.logger.ErrorContext(r.Context(), "operator billing request failed",
		"path", r.URL.Path,
		"error", err.Error(),
	)
	return common.OperatorInternal(internalErrorMessage)
}
