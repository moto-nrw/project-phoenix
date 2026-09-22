package operator

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBillingReport answers the billing routes without a database; the
// capture and the store are covered by the compose tests.
type fakeBillingReport struct {
	counts    []organizationtenancy.BillingKeyDateCount
	listErr   error
	setDays   []int
	setResult organizationtenancy.BillingKeyDay
}

func (f *fakeBillingReport) BillingKeyDay(context.Context) (organizationtenancy.BillingKeyDay, error) {
	return f.setResult, nil
}

func (f *fakeBillingReport) SetBillingKeyDay(_ context.Context, keyDay int, _ int64) (organizationtenancy.BillingKeyDay, error) {
	f.setDays = append(f.setDays, keyDay)
	if keyDay < 1 || keyDay > 28 {
		return organizationtenancy.BillingKeyDay{}, organizationtenancy.ErrInvalidBillingKeyDay
	}
	return organizationtenancy.BillingKeyDay{Day: keyDay, NextKeyDate: "2026-10-10"}, nil
}

func (f *fakeBillingReport) ListBillingKeyDateCounts(context.Context) ([]organizationtenancy.BillingKeyDateCount, error) {
	return f.counts, f.listErr
}

func (f *fakeBillingReport) RecordDueBillingKeyDates(context.Context, time.Time) (int, error) {
	return 0, nil
}

func billingCount(period, keyDate, school string) organizationtenancy.BillingKeyDateCount {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return organizationtenancy.BillingKeyDateCount{
		SchoolID: 7, SchoolName: school, OrganizationName: "Träger Nord",
		Period: period, KeyDate: keyDate, ActiveStudents: 120, ActiveTerminals: 3,
		RecordedAt: time.Date(2026, time.August, 15, 6, 5, 0, 0, berlin),
	}
}

func TestBillingExportWritesAGermanSpreadsheetCSV(t *testing.T) {
	t.Parallel()
	report := &fakeBillingReport{counts: []organizationtenancy.BillingKeyDateCount{
		billingCount("2026-08-01", "2026-08-15", "=HYPERLINK(\"x\")"),
		billingCount("2026-07-01", "2026-07-15", "OGS Juli"),
	}}
	resource := NewBillingResource(report, nil)

	recorder := httptest.NewRecorder()
	resource.ExportKeyDateCounts(recorder, httptest.NewRequest(http.MethodGet, "/billing/key-date-counts/export?month=2026-08", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/csv; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="abrechnung-stichtag-2026-08.csv"`, recorder.Header().Get("Content-Disposition"))
	body := recorder.Body.String()
	require.True(t, strings.HasPrefix(body, "\xEF\xBB\xBF"), "Excel needs the BOM to read umlauts")
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(body, "\xEF\xBB\xBF")), "\n")
	require.Len(t, lines, 2, "only the requested month")
	assert.Equal(t, "Monat;Stichtag;Träger;Schule;Schul-ID;Aktive Kinder;Aktive Terminals;Erfasst am", lines[0])
	assert.Equal(t, `08/2026;15.08.2026;Träger Nord;"'=HYPERLINK(""x"")";7;120;3;15.08.2026 06:05`, lines[1])
}

func TestBillingExportWithoutMonthContainsEveryMonth(t *testing.T) {
	t.Parallel()
	report := &fakeBillingReport{counts: []organizationtenancy.BillingKeyDateCount{
		billingCount("2026-08-01", "2026-08-15", "OGS August"),
		billingCount("2026-07-01", "2026-07-15", "OGS Juli"),
	}}
	recorder := httptest.NewRecorder()
	NewBillingResource(report, nil).ExportKeyDateCounts(recorder, httptest.NewRequest(http.MethodGet, "/billing/key-date-counts/export", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, `attachment; filename="abrechnung-stichtage.csv"`, recorder.Header().Get("Content-Disposition"))
	assert.Contains(t, recorder.Body.String(), "OGS August")
	assert.Contains(t, recorder.Body.String(), "OGS Juli")
}

func TestBillingExportRejectsAMalformedMonth(t *testing.T) {
	t.Parallel()
	for _, month := range []string{"2026-13", "2026-8", "08-2026", "x"} {
		recorder := httptest.NewRecorder()
		NewBillingResource(&fakeBillingReport{}, nil).ExportKeyDateCounts(recorder,
			httptest.NewRequest(http.MethodGet, "/billing/key-date-counts/export?month="+month, nil))
		assert.Equal(t, http.StatusBadRequest, recorder.Code, month)
	}
}

func TestBillingListHidesInternalErrors(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	report := &fakeBillingReport{listErr: errors.New("pq: relation does not exist")}
	NewBillingResource(report, nil).ListKeyDateCounts(recorder, httptest.NewRequest(http.MethodGet, "/billing/key-date-counts", nil))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "relation")
}

func TestBillingKeyDayUpdateValidatesTheBody(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		body   string
		status int
	}{
		{"missing key day", `{}`, http.StatusBadRequest},
		{"day outside 1..28", `{"key_day":31}`, http.StatusBadRequest},
		{"valid day", `{"key_day":10}`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := &fakeBillingReport{}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/billing/key-day", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			NewBillingResource(report, nil).UpdateKeyDay(recorder, request)
			assert.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			if tc.status == http.StatusOK {
				assert.Equal(t, []int{10}, report.setDays)
				assert.Contains(t, recorder.Body.String(), `"key_day":10`)
			}
		})
	}
}
