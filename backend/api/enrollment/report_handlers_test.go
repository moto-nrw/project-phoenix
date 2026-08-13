package enrollment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

func TestParseClassRosterExportRequestAcceptsNumericPhaseID(t *testing.T) {
	req := httptest.NewRequest("POST", "/enrollment/admin/reports/class-roster/export", strings.NewReader(`{
		"format":"xlsx",
		"filters":{"phase_id":42,"school_class":" 1a "}
	}`))

	format, filters, err := parseClassRosterExportRequest(req)

	require.NoError(t, err)
	assert.Equal(t, listexport.FormatXLSX, format)
	assert.Equal(t, int64(42), filters.PhaseID)
	assert.Equal(t, "1a", filters.SchoolClass)
}

func TestParseClassRosterExportRequestRejectsMissingClass(t *testing.T) {
	req := httptest.NewRequest("POST", "/enrollment/admin/reports/class-roster/export", strings.NewReader(`{
		"format":"pdf",
		"filters":{"phase_id":"42"}
	}`))

	_, _, err := parseClassRosterExportRequest(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "school_class is required")
}

func TestClassRosterExportRequiresConfigManageAndUsersRead(t *testing.T) {
	testutil.SeedTestJWTConfig()
	router := (&Resource{
		ReportService:     &fakeClassRosterReportService{},
		ListExportService: listexport.NewService(),
		runInTenantTxForTest: func(r *http.Request, fn func(context.Context) error) error {
			return fn(r.Context())
		},
	}).Router()
	body := `{"format":"xlsx","filters":{"phase_id":42,"school_class":"1a"}}`

	configOnly := newClassRosterExportHTTPRequest(t, body, []string{permissions.ConfigManage})
	configOnlyRecorder := httptest.NewRecorder()
	router.ServeHTTP(configOnlyRecorder, configOnly)
	assert.Equal(t, http.StatusForbidden, configOnlyRecorder.Code)

	configAndUsersRead := newClassRosterExportHTTPRequest(t, body, []string{permissions.ConfigManage, permissions.UsersRead})
	configAndUsersReadRecorder := httptest.NewRecorder()
	router.ServeHTTP(configAndUsersReadRecorder, configAndUsersRead)
	assert.Equal(t, http.StatusOK, configAndUsersReadRecorder.Code)
}

func TestBuildClassRosterTableDocumentRendersPhaseAwareCells(t *testing.T) {
	report := &enrollmentService.ClassRosterReport{
		Phase: enrollmentService.CareUsagePhase{ID: 42, Name: "Schuljahr 2026"},
		Filters: enrollmentService.ClassRosterAppliedFilters{
			PhaseID:     42,
			SchoolClass: "1a",
			Status:      enrollmentModels.ChildStatusApproved,
		},
		Totals: enrollmentService.ClassRosterTotals{Students: 2, Registered: 1},
		Rows: []enrollmentService.ClassRosterRow{
			{
				FirstName:         "Lina",
				LastName:          "Muster",
				SchoolClass:       "1a",
				GroupName:         "Eulen",
				Registered:        true,
				EnrollmentSummary: "Angemeldet: Randstunde",
				CareDays:          []string{"mon", "wed"},
				OfferingsByDay: map[string][]string{
					"mon": []string{"Randstunde"},
					"wed": []string{"Ganztag"},
				},
				ArrivalByDay: map[string]string{"mon": "11:30"},
				PickupByDay:  map[string]string{"mon": "14:30"},
				Departure:    "Mo: Abholung",
				Guardians: []enrollmentService.ClassRosterGuardian{
					{Name: "Eva Muster", Email: "eva@example.test", Phone: "02551 123"},
				},
			},
			{
				FirstName:         "Tom",
				LastName:          "Ohne",
				SchoolClass:       "1a",
				EnrollmentSummary: "Keine Anmeldung",
				CareDays:          []string{},
				OfferingsByDay:    map[string][]string{},
				ArrivalByDay:      map[string]string{},
				PickupByDay:       map[string]string{},
				Departure:         "Geht alleine",
				Guardians: []enrollmentService.ClassRosterGuardian{
					{Name: "Stamm Kontakt", Phone: "02551 456"},
				},
			},
		},
	}

	doc := buildClassRosterTableDocument(report)

	assert.Equal(t, "Klassenliste 1a - Schuljahr 2026", doc.Title)
	assert.Equal(t, "2 Kinder, 1 angemeldet", doc.Subtitle)
	assert.Equal(t, []listexport.Column{
		{ID: listexport.ColumnName, Label: "Name"},
		{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
		{ID: listexport.ColumnWeeklyMonday, Label: "Montag"},
		{ID: listexport.ColumnWeeklyTuesday, Label: "Dienstag"},
		{ID: listexport.ColumnWeeklyWednesday, Label: "Mittwoch"},
		{ID: listexport.ColumnWeeklyThursday, Label: "Donnerstag"},
		{ID: listexport.ColumnWeeklyFriday, Label: "Freitag"},
		{ID: listexport.ColumnDeparture, Label: "Geh-/Abholweise"},
		{ID: listexport.ColumnGuardianContacts, Label: "Erziehungsberechtigte"},
	}, doc.Columns)
	require.Len(t, doc.Rows, 2)
	assert.Empty(t, doc.Rows[0].Values[listexport.ColumnEnrollmentSummary])
	assert.Empty(t, doc.Rows[0].Values[listexport.ColumnGroup])
	assert.Empty(t, doc.Rows[0].Values[listexport.ColumnCareDays])
	assert.Equal(t, "14:30 Uhr", doc.Rows[0].Values[listexport.ColumnWeeklyMonday])
	assert.Equal(t, "Betreuung", doc.Rows[0].Values[listexport.ColumnWeeklyWednesday])
	assert.Equal(t, "Eva Muster (eva@example.test, 02551 123)", doc.Rows[0].Values[listexport.ColumnGuardianContacts])
	assert.Empty(t, doc.Rows[1].Values[listexport.ColumnEnrollmentSummary])
	assert.Equal(t, "—", doc.Rows[1].Values[listexport.ColumnWeeklyMonday])
	assert.Equal(t, "Stamm Kontakt (02551 456)", doc.Rows[1].Values[listexport.ColumnGuardianContacts])
}

func TestClassRosterWeeklyCellShowsOnlyPickupTimeForCareDays(t *testing.T) {
	tests := []struct {
		name string
		row  enrollmentService.ClassRosterRow
		want string
	}{
		{
			name: "care with pickup time",
			row: enrollmentService.ClassRosterRow{
				CareDays:       []string{"mon"},
				OfferingsByDay: map[string][]string{"mon": {"Ganztagsbetreuung"}},
				PickupByDay:    map[string]string{"mon": "14:30"},
			},
			want: "14:30 Uhr",
		},
		{
			name: "care without pickup time",
			row: enrollmentService.ClassRosterRow{
				CareDays:       []string{"mon"},
				OfferingsByDay: map[string][]string{"mon": {"Randstunde"}},
			},
			want: "Betreuung",
		},
		{
			name: "multiple offerings",
			row: enrollmentService.ClassRosterRow{
				CareDays:       []string{"mon"},
				OfferingsByDay: map[string][]string{"mon": {"Ganztag", "Randstunde"}},
				PickupByDay:    map[string]string{"mon": "16:00"},
			},
			want: "16:00 Uhr",
		},
		{
			name: "no care ignores stale pickup time",
			row: enrollmentService.ClassRosterRow{
				PickupByDay: map[string]string{"mon": "14:30"},
			},
			want: "—",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, classRosterWeeklyCell(tt.row, "mon"))
		})
	}
}

func newClassRosterExportHTTPRequest(t *testing.T, body string, perms []string) *http.Request {
	t.Helper()
	token := testutil.MintTestJWT(t, jwt.AppClaims{
		ID:          100,
		Sub:         "admin@example.test",
		TenantID:    200,
		Roles:       []string{"admin"},
		Permissions: perms,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/reports/class-roster/export", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

type fakeClassRosterReportService struct {
	enrollmentService.ReportService
}

func (s *fakeClassRosterReportService) ExportClassRoster(_ context.Context, filters enrollmentService.ClassRosterFilters, _ int64, _, _ string) (*enrollmentService.ClassRosterReport, error) {
	return &enrollmentService.ClassRosterReport{
		Phase: enrollmentService.CareUsagePhase{ID: filters.PhaseID, Name: "Schuljahr 2026"},
		Filters: enrollmentService.ClassRosterAppliedFilters{
			PhaseID:     filters.PhaseID,
			SchoolClass: filters.SchoolClass,
			Status:      enrollmentModels.ChildStatusApproved,
		},
		Totals: enrollmentService.ClassRosterTotals{Students: 1, Registered: 1},
		Rows: []enrollmentService.ClassRosterRow{
			{
				FirstName:         "Lina",
				LastName:          "Muster",
				SchoolClass:       filters.SchoolClass,
				Registered:        true,
				EnrollmentSummary: "Angemeldet",
				CareDays:          []string{"mon"},
				ArrivalByDay:      map[string]string{},
				PickupByDay:       map[string]string{},
				Departure:         "Abholung",
			},
		},
	}, nil
}
