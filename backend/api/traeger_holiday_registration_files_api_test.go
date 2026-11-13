// Router-level tests for the files of the holiday registrations (#6691, ADR
// 0118 addendum): the Träger-Büro downloads one registration as a PDF and
// the registrations of a period as a ZIP with one PDF each. Both need the
// right "Anmeldung verwalten", stay with their Träger, stand in the audit
// trail and name health information only with the right to see it.
package api

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

func (f reviewFixture) file(t *testing.T, token, suffix string) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodGet, f.path(suffix), nil)
}

func TestTraegerDownloadsAHolidayRegistrationAsPDF(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Anmeldung PDF")

	rec := f.file(t, f.office.token, fmt.Sprintf("/%d/pdf", f.siblings))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, fmt.Sprintf(`attachment; filename="ferienanmeldung-%d-muster.pdf"`, f.siblings), rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")), "the answer is a PDF")
	assert.Equal(t, []string{fmt.Sprintf("read:%d", f.siblings)}, f.auditActions(t), "the download stands in the trail as a read")

	// The health information needs the right to see it, and its read stands
	// in the trail like the one on the screen.
	rec = f.file(t, f.office.token, fmt.Sprintf("/%d/pdf?health=true", f.siblings))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{
		fmt.Sprintf("read:%d", f.siblings), fmt.Sprintf("read:%d", f.siblings), fmt.Sprintf("health:%d", f.siblings),
	}, f.auditActions(t))

	rec = f.file(t, f.office.token, "/999999999/pdf")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestHolidayRegistrationFilesKeepAnsweredInactiveAndArchivedCriteria(t *testing.T) {
	t.Parallel()
	f := newAllocationFixture(t, "Träger PDF alte Kriterien")
	first := f.submitCriteria(t, "alte.kriterien@example.org", true, false)
	// Keep the family criterion disabled and archive the child criterion.
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, "/traeger/holiday-periods/criteria", map[string]any{"criteria": []map[string]any{
		{"id": f.working, "name": "Berufstätig", "question": "Sind Sie berufstätig?", "scope": "family", "active": false},
	}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.publicOffer(t).Criteria, "new registrations no longer see the old questions")
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
		placesRegistration(f.reviewFixture, "ohne.kriterien@example.org", map[string][]int{"Tom": {0}}))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	later := decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())

	_, _, _, records := enrollmentCompose.NewHolidayRegistrationReview(testHealthCipher(), nil, nil, owners.HolidayTestSchoolCalendar{}, nil)
	reviewer := enrollment.HolidayReviewer{
		OrganizationID: f.office.organization,
		AccountID:      testpkg.CreateTestAccount(t, f.db, "holiday-records").ID,
	}
	ctx := testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, f.db))
	require.NoError(t, testpkg.WithinTraeger(ctx, reviewer.OrganizationID, func(ctx context.Context) error {
		// Both downloads share these records, before the PDF renderer.
		single, err := records.HolidayRegistrationRecord(ctx, reviewer, f.period.ID, first.ID, false)
		require.NoError(t, err)
		require.Len(t, single.Records, 1)
		criteria := single.Records[0].Detail.Criteria
		require.Len(t, criteria, 2, "both stored answers survive deactivation and archiving")
		assert.Equal(t, f.working, criteria[0].CriterionID)
		assert.Equal(t, "Sind Sie berufstätig?", criteria[0].Question)
		assert.Nil(t, criteria[0].ChildID)
		assert.Equal(t, enrollment.HolidayCriterionYes, criteria[0].Stated)
		assert.Equal(t, f.sibling, criteria[1].CriterionID)
		assert.Equal(t, "Ist ein Geschwisterkind schon in der OGS?", criteria[1].Question)
		require.NotNil(t, criteria[1].ChildID)
		assert.Equal(t, single.Records[0].Detail.Children[0].ID, *criteria[1].ChildID)
		assert.Equal(t, enrollment.HolidayCriterionNo, criteria[1].Stated)

		archive, err := records.HolidayRegistrationRecords(ctx, reviewer, f.period.ID, enrollment.HolidayRegistrationFilter{}, false)
		require.NoError(t, err)
		require.Len(t, archive.Records, 2)
		for _, record := range archive.Records {
			if record.Detail.ID == first.ID {
				assert.Equal(t, criteria, record.Detail.Criteria, "the ZIP keeps the same answers as the single PDF")
			} else {
				assert.Equal(t, later.ID, record.Detail.ID)
				assert.Empty(t, record.Detail.Criteria, "another family's historical questions do not appear")
			}
		}
		return nil
	}))
}

func TestTraegerDownloadsTheHolidayRegistrationsOfAPeriodAsZIP(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Anmeldungen ZIP")
	require.Equal(t, http.StatusOK, f.act(t, f.again, "confirm").Code)
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET guardian_last_name = 'Müller-Çelik / Süß'
		WHERE id = ?`, f.again)
	require.NoError(t, err)
	trail := len(f.auditActions(t))

	entries := func(rec *httptest.ResponseRecorder) []string {
		t.Helper()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		require.NoError(t, err)
		names := make([]string, len(archive.File))
		for i, file := range archive.File {
			names[i] = file.Name
			content, err := file.Open()
			require.NoError(t, err)
			head := make([]byte, 5)
			_, err = content.Read(head)
			require.NoError(t, err)
			require.NoError(t, content.Close())
			assert.Equal(t, "%PDF-", string(head), "%s is a PDF", file.Name)
		}
		return names
	}

	rec := f.file(t, f.office.token, "/pdf")
	assert.Equal(t, []string{
		fmt.Sprintf("ferienanmeldung-%d-muster.pdf", f.siblings),
		fmt.Sprintf("ferienanmeldung-%d-mueller-celik-suess.pdf", f.again),
	}, entries(rec), "one file per registration, named in plain ASCII with its number")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `attachment; filename="ferienanmeldungen-`)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `.zip"`)

	assert.Equal(t, []string{fmt.Sprintf("ferienanmeldung-%d-mueller-celik-suess.pdf", f.again)},
		entries(f.file(t, f.office.token, "/pdf?status=confirmed")), "the archive follows the filter of the list")
	assert.Empty(t, entries(f.file(t, f.office.token, "/pdf?status=cancelled")), "a filter without a registration answers an empty archive")
	assert.Equal(t, []string{"export", "export", "export"}, f.auditActions(t)[trail:], "every download stands in the trail as an export")

	entries(f.file(t, f.office.token, "/pdf?health=true"))
	assert.Equal(t, []string{"export", fmt.Sprintf("health:%d", f.siblings), fmt.Sprintf("health:%d", f.again)},
		f.auditActions(t)[trail+3:], "with health information the trail names the health read of every registration")

	rec = f.file(t, f.office.token, "/pdf?status=unknown")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestHolidayRegistrationFilesNeedTheRightsAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Grenze Dateien")
	readOnly := newTraegerOffice(t, f.db, f.traeger, "Träger Dateien Lesend", identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Dateien Fremd", identityaccess.TraegerRoleAdministration)

	for _, suffix := range []string{"/pdf", fmt.Sprintf("/%d/pdf", f.siblings), "/pdf?health=true", fmt.Sprintf("/%d/pdf?health=true", f.siblings)} {
		rec := f.file(t, readOnly.token, suffix)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right: %s", suffix, rec.Body.String())
		rec = f.file(t, foreign.token, suffix)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s of another Träger: %s", suffix, rec.Body.String())
	}

	// "Anmeldung verwalten" without "Gesundheitsangaben sehen": the files
	// without health information only. No role bundles the rights that way,
	// so a route-sized router binds such a principal of the office's Träger.
	f.list(t, "")
	var account int64
	require.NoError(t, f.db.NewRaw(`SELECT actor_account_id FROM enrollment.holiday_registration_audit
		WHERE holiday_period_id = ? LIMIT 1`, f.period.ID).Scan(t.Context(), &account))
	withoutHealth := holidayFilesRouter(t, f, account, identityaccess.TraegerRightEnrollmentManage)
	for _, suffix := range []string{"/pdf", fmt.Sprintf("/%d/pdf", f.siblings)} {
		assert.Equal(t, http.StatusOK, traegerJSON(t, withoutHealth, "", http.MethodGet, f.path(suffix), nil).Code, "%s without health information", suffix)
		rec := traegerJSON(t, withoutHealth, "", http.MethodGet, f.path(suffix+"?health=true"), nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s with health information without the right: %s", suffix, rec.Body.String())
	}
	for _, action := range f.auditActions(t) {
		assert.NotContains(t, action, "health", "a refused request reads no health information")
	}
}

// unnamedHolidaySchools names no school: the route-sized router has no
// Träger projection.
type unnamedHolidaySchools struct{}

func (unnamedHolidaySchools) HolidaySchoolNames(context.Context, int64) (map[int64]string, error) {
	return map[int64]string{}, nil
}

// holidayFilesRouter mounts the holiday period routes for an authenticated
// member of the fixture's Träger with exactly the given rights.
func holidayFilesRouter(t *testing.T, f reviewFixture, account int64, rights ...string) http.Handler {
	t.Helper()
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: account, OrganizationID: f.office.organization, Scope: tenant.ScopeOrg, Permissions: rights,
	})
	require.NoError(t, err)
	review, places, allocation, records := enrollmentCompose.NewHolidayRegistrationReview(testHealthCipher(), nil, nil, owners.HolidayTestSchoolCalendar{}, nil)
	_, wishes := enrollmentCompose.NewHolidayWishes(testHealthCipher())
	resource := enrollmentHTTP.NewHolidayPeriodResource(enrollmentCompose.NewHolidayPeriods(nil, owners.HolidayTestSchoolCalendar{}),
		review, records, wishes, places, allocation, allocation, unnamedHolidaySchools{},
		holidayKnownChildren{}, enrollmentCompose.NewHolidayKnownChildTrail(),
		common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage), common.RequiresPermission(identityaccess.TraegerRightHealthRead))
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope := tenant.ClaimScope{}.BindOrganization(tenant.WithUnitOfWork(r.Context(), testpkg.TenantRuntime(t, f.db)), f.office.organization, tenant.ScopeOrg)
			next.ServeHTTP(w, r.WithContext(permissions.WithPrincipal(scope, principal)))
		})
	})
	router.Mount("/traeger/holiday-periods", resource.TraegerRouter(func(r chi.Router, mount func(chi.Router, common.Middleware)) { mount(r, common.TraegerTxMiddleware) }))
	return router
}

func TestTraegerHolidayRegistrationArchiveQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Dateien")
	archive := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, f.path("/pdf?health=true"), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	two := archive()
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), siblingsRegistration(f.offer, "dateien@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Len(t, archive().Statements, len(two.Statements), "the archive reads the same statements for any number of registrations")
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_registration_files", two)
}

// TestHolidayRegistrationKeepsTheRetentionOfItsText pins the retention a
// Träger sets (#6691): the offer names it in the privacy text, a
// registration keeps the days it agreed to, and a parent whose form still
// names the old retention is told that the form changed.
func TestHolidayRegistrationKeepsTheRetentionOfItsText(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Frist Einstellung")
	setRetention := func(days any) *httptest.ResponseRecorder {
		return traegerJSON(t, f.traeger, f.office.token, http.MethodPut,
			"/traeger/settings/values/"+configModels.KeyHolidayRegistrationRetentionDays, map[string]any{"value": days})
	}
	for _, days := range []int{91, 1096} {
		assert.Equal(t, http.StatusBadRequest, setRetention(days).Code, "%d days are outside the bounds", days)
	}
	require.Equal(t, http.StatusOK, setRetention(180).Code)

	rec := publicJSON(t, f.public, http.MethodGet, registrationPath(f.period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	offer := decodeSchoolPortalData[struct {
		ConsentTexts struct {
			Version        int    `json:"version"`
			RetentionDays  int    `json:"retention_days"`
			PrivacyConsent string `json:"privacy_consent"`
		} `json:"consent_texts"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, 5, offer.ConsentTexts.Version)
	assert.Equal(t, 180, offer.ConsentTexts.RetentionDays)
	assert.Contains(t, offer.ConsentTexts.PrivacyConsent, "Er löscht sie 180 Tage nach den Ferien.")
	assert.Contains(t, offer.ConsentTexts.PrivacyConsent, "Gesundheitsangaben löscht er schon 92 Tage nach den Ferien.")

	stale := siblingsRegistration(f.offer, "alte.frist@example.org")
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), stale)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_form_changed", code, "a form with the old retention is told the form changed")
	assert.Equal(t, []string{"retention_days"}, fields)

	agreed := siblingsRegistration(f.offer, "neue.frist@example.org")
	agreed["retention_days"] = 180
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), agreed)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	later := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID

	// The last week of the fixture ends on 15.01.2027. The earlier
	// registrations keep their year, whatever the Träger sets later.
	lastDay := calendar.NewDate(2027, 1, 15)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := decodeSchoolPortalData[struct {
		Registrations []struct {
			ID              int64  `json:"id"`
			RetentionDays   int    `json:"retention_days"`
			DeletesOn       string `json:"deletes_on"`
			HealthDeletesOn string `json:"health_deletes_on"`
		} `json:"registrations"`
	}](t, rec.Body.Bytes())
	require.Len(t, list.Registrations, 3)
	for _, registration := range list.Registrations {
		days := holidayDefaultRetentionDays
		if registration.ID == later {
			days = 180
		}
		assert.Equal(t, days, registration.RetentionDays, "registration %d", registration.ID)
		assert.Equal(t, lastDay.AddDays(days+1).String(), registration.DeletesOn, "the day the retention deletes registration %d", registration.ID)
		assert.Equal(t, lastDay.AddDays(holidayLegacyRetentionDays+1).String(), registration.HealthDeletesOn,
			"the health information goes after 92 days whatever the registration keeps")
	}

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(later, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Er löscht sie 180 Tage nach den Ferien.", "the detail names the text the parents agreed to")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Er löscht sie 365 Tage nach den Ferien.")
}
