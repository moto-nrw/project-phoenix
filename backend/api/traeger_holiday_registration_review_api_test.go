// Router-level tests for the review of the holiday registrations by the
// Träger-Büro (#4663, ADR 0114): the list with filters and duplicate hints,
// confirming, cancelling, rebooking, freezing and reopening with history,
// the CSV and Excel export, the audit trail of every read and change, the
// right "Anmeldung verwalten" and the Träger boundary.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type reviewBooking struct {
	WeekID        int64  `json:"week_id"`
	HostSchoolID  int64  `json:"host_school_id"`
	TimeModelID   int64  `json:"time_model_id"`
	TimeModelName string `json:"time_model_name"`
	PriceCents    int64  `json:"price_cents"`
}

type reviewChild struct {
	ID                       int64           `json:"id"`
	FirstName                string          `json:"first_name"`
	HomeSchoolID             *int64          `json:"home_school_id"`
	GuestSchoolName          string          `json:"guest_school_name"`
	Bookings                 []reviewBooking `json:"bookings"`
	DuplicateRegistrationIDs []int64         `json:"duplicate_registration_ids"`
}

type reviewRegistration struct {
	ID       int64         `json:"id"`
	Status   string        `json:"status"`
	Frozen   bool          `json:"frozen"`
	Children []reviewChild `json:"children"`
	Health   []struct {
		ChildID   int64  `json:"child_id"`
		Allergies string `json:"allergies"`
	} `json:"health"`
	Contacts []struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"contacts"`
	History []struct {
		Action         string         `json:"action"`
		ActorAccountID *int64         `json:"actor_account_id"`
		Details        map[string]any `json:"details"`
	} `json:"history"`
}

type reviewList struct {
	Registrations []reviewRegistration `json:"registrations"`
	Total         int                  `json:"total"`
	StatusCounts  map[string]int       `json:"status_counts"`
}

// reviewFixture is a published period of a Träger with two registrations:
// the siblings Lia (first school, week 1 "bis 16 Uhr", with an allergy) and
// Ben (holiday guest, both weeks "bis 13 Uhr"), and a second registration of
// Lia (first school, week 2), a possible duplicate.
type reviewFixture struct {
	holidayRegistrationFixture
	office   traegerOffice
	period   traegerPeriodLink
	offer    holidayOfferBody
	siblings int64
	again    int64
}

func newReviewFixture(t *testing.T, name string) reviewFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	submit := func(body map[string]any) int64 {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[struct {
			ID int64 `json:"id"`
		}](t, rec.Body.Bytes()).ID
	}
	siblings := submit(siblingsRegistration(offer, "eva.muster@example.org"))
	again := siblingsRegistration(offer, "papa.muster@example.org")
	lia := again["children"].([]map[string]any)[0]
	lia["first_name"], lia["last_name"] = "LIA", "muster"
	lia["allergies"] = ""
	lia["bookings"] = []map[string]any{{"week_id": offer.Weeks[1].ID, "time_model_id": offer.Weeks[1].TimeModels[0].ID}}
	again["children"] = []map[string]any{lia}
	return reviewFixture{
		holidayRegistrationFixture: f, office: office, period: period, offer: offer,
		siblings: siblings, again: submit(again),
	}
}

func (f reviewFixture) path(suffix string) string {
	return holidayPeriodPath(f.period.ID) + "/registrations" + suffix
}

func (f reviewFixture) registrationPath(id int64, suffix string) string {
	return f.path(fmt.Sprintf("/%d%s", id, suffix))
}

func (f reviewFixture) list(t *testing.T, query string) reviewList {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(query), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[reviewList](t, rec.Body.Bytes())
}

// act changes a registration. A confirmation freezes the contribution, so
// the Elternabrechnung serves it (#4664, ADR 0115); its answer names the
// registration and its status like the other changes.
func (f reviewFixture) act(t *testing.T, id int64, action string) *httptest.ResponseRecorder {
	t.Helper()
	if action == "confirm" {
		return traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, id), nil)
	}
	return traegerJSON(t, f.traeger, f.office.token, http.MethodPost, f.registrationPath(id, "/"+action), nil)
}

func (f reviewFixture) detail(t *testing.T, rec *httptest.ResponseRecorder) reviewRegistration {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[reviewRegistration](t, rec.Body.Bytes())
}

// auditActions lists the audit trail of the period, oldest first, as
// "action" or "action:registration".
func (f reviewFixture) auditActions(t *testing.T) []string {
	t.Helper()
	var rows []struct {
		Action         string `bun:"action"`
		RegistrationID *int64 `bun:"holiday_registration_id"`
		Actor          *int64 `bun:"actor_account_id"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT action, holiday_registration_id, actor_account_id
		FROM enrollment.holiday_registration_audit WHERE holiday_period_id = ? ORDER BY id`, f.period.ID).Scan(context.Background(), &rows))
	actions := make([]string, len(rows))
	for i, row := range rows {
		require.NotNil(t, row.Actor, "every entry names the person of the Träger-Büro")
		actions[i] = row.Action
		if row.RegistrationID != nil {
			actions[i] = fmt.Sprintf("%s:%d", row.Action, *row.RegistrationID)
		}
	}
	return actions
}

func registrationByID(t *testing.T, list reviewList, id int64) reviewRegistration {
	t.Helper()
	for _, registration := range list.Registrations {
		if registration.ID == id {
			return registration
		}
	}
	require.Failf(t, "registration missing", "registration %d is not in the list", id)
	return reviewRegistration{}
}

func registrationIDs(list reviewList) []int64 {
	ids := []int64{}
	for _, registration := range list.Registrations {
		ids = append(ids, registration.ID)
	}
	return ids
}

func TestTraegerListsHolidayRegistrationsWithFiltersAndDuplicateHints(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Liste Ferien")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Erdnüsse", "the list carries no health information")
	all := decodeSchoolPortalData[reviewList](t, rec.Body.Bytes())
	assert.Equal(t, 2, all.Total)
	assert.Equal(t, map[string]int{"received": 2}, all.StatusCounts)
	assert.Equal(t, []int64{f.siblings, f.again}, registrationIDs(all))
	siblings := registrationByID(t, all, f.siblings)
	assert.False(t, siblings.Frozen, "the registration window is still open")
	require.Len(t, siblings.Children, 2)
	assert.Equal(t, []int64{f.again}, siblings.Children[0].DuplicateRegistrationIDs,
		"Lia was registered twice: same name in other letter case and the same date of birth")
	assert.Empty(t, siblings.Children[1].DuplicateRegistrationIDs)
	assert.Nil(t, siblings.Children[1].HomeSchoolID, "Ben is a holiday guest")
	require.Len(t, siblings.Children[1].Bookings, 2)
	assert.Equal(t, "bis 13 Uhr", siblings.Children[1].Bookings[0].TimeModelName)
	assert.Equal(t, f.office.schools[1], siblings.Children[1].Bookings[1].HostSchoolID, "the second week is hosted by the second school")
	assert.Equal(t, []int64{f.siblings}, registrationByID(t, all, f.again).Children[0].DuplicateRegistrationIDs)

	assert.Equal(t, []int64{f.siblings}, registrationIDs(f.list(t, "?school=guest")))
	assert.Equal(t, []int64{f.siblings, f.again}, registrationIDs(f.list(t, fmt.Sprintf("?school=%d", f.office.schools[0]))))
	assert.Empty(t, registrationIDs(f.list(t, fmt.Sprintf("?school=%d", f.office.schools[1]))), "no child attends the second school")
	assert.Equal(t, []int64{f.siblings, f.again}, registrationIDs(f.list(t, fmt.Sprintf("?week_id=%d", f.offer.Weeks[1].ID))))
	assert.Equal(t, []int64{f.siblings}, registrationIDs(f.list(t, "?time_model=bis+16+Uhr")))
	assert.Equal(t, []int64{f.again}, registrationIDs(f.list(t, fmt.Sprintf("?school=%d&week_id=%d", f.office.schools[0], f.offer.Weeks[1].ID))),
		"school, week and time model must match the same child")
	none := f.list(t, "?status=confirmed")
	assert.Empty(t, none.Registrations)
	assert.Equal(t, 2, none.Total, "the counts always cover the whole period")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path("?status=unknown"), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	assert.Equal(t, []string{"list", "list", "list", "list", "list", "list", "list", "list"}, f.auditActions(t),
		"every list read is in the audit trail; the refused filter read nothing")
}

func TestTraegerConfirmsAndCancelsHolidayRegistrationsWithHistory(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bestätigung Ferien")

	detail := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil))
	require.Len(t, detail.Health, 1, "the detail shows the health information to the right")
	assert.Equal(t, "Erdnüsse", detail.Health[0].Allergies)
	assert.Equal(t, detail.Children[0].ID, detail.Health[0].ChildID)
	require.Len(t, detail.Contacts, 2)
	assert.Empty(t, detail.History)

	confirmed := f.detail(t, f.act(t, f.siblings, "confirm"))
	assert.Equal(t, "confirmed", confirmed.Status)
	rec := f.act(t, f.siblings, "confirm")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_state_conflict", code)

	cancelled := f.detail(t, f.act(t, f.again, "cancel"))
	assert.Equal(t, "cancelled", cancelled.Status)
	assert.Empty(t, cancelled.Children[0].DuplicateRegistrationIDs, "a cancelled registration is no duplicate hint")
	rec = f.act(t, f.again, "confirm")
	require.Equal(t, http.StatusConflict, rec.Code, "a cancelled registration cannot be confirmed: %s", rec.Body.String())
	rec = f.act(t, f.again, "cancel")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	list := f.list(t, "")
	assert.Equal(t, map[string]int{"confirmed": 1, "cancelled": 1}, list.StatusCounts)
	assert.Empty(t, registrationByID(t, list, f.siblings).Children[0].DuplicateRegistrationIDs,
		"the cancelled second registration no longer counts as a duplicate")

	cancelledLater := f.detail(t, f.act(t, f.siblings, "cancel"))
	require.Len(t, cancelledLater.History, 2)
	assert.Equal(t, "confirm", cancelledLater.History[0].Action)
	assert.Equal(t, "cancel", cancelledLater.History[1].Action)
	assert.Equal(t, "confirmed", cancelledLater.History[1].Details["from"])
	require.NotNil(t, cancelledLater.History[0].ActorAccountID)

	// A registration of another period of the same Träger, or none at all,
	// answers 404 for the read and is not written to the audit trail.
	other := januaryHolidays(f.office)
	other.Name = "Osterferien 2027"
	otherPeriod := createHolidayPeriod(t, f.traeger, f.office, other)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		fmt.Sprintf("%s/registrations/%d", holidayPeriodPath(otherPeriod.ID), f.siblings), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_entry_not_found", code)

	rec = f.act(t, f.siblings+f.again+1000, "confirm")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_entry_not_found", code)

	assert.Equal(t, []string{
		fmt.Sprintf("read:%d", f.siblings), fmt.Sprintf("confirm:%d", f.siblings), fmt.Sprintf("cancel:%d", f.again),
		"list", fmt.Sprintf("cancel:%d", f.siblings),
	}, f.auditActions(t), "reads and changes are in the audit trail; refused changes left nothing")
}

func TestTraegerFreezesReopensAndRebooksHolidayRegistrations(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Einfrieren Ferien")
	ben := f.list(t, "").Registrations[0].Children[1]

	frozen := f.detail(t, f.act(t, f.siblings, "freeze"))
	assert.True(t, frozen.Frozen)
	rec := f.act(t, f.siblings, "freeze")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	rec = f.act(t, f.siblings, "cancel")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_frozen", code)
	rebook := map[string]any{"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[0].TimeModels[1].ID}}}
	bookingsPath := f.registrationPath(f.siblings, fmt.Sprintf("/children/%d/bookings", ben.ID))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, bookingsPath, rebook)
	require.Equal(t, http.StatusConflict, rec.Code, "a frozen registration is not rebooked: %s", rec.Body.String())
	confirmed := f.detail(t, f.act(t, f.siblings, "confirm"))
	assert.Equal(t, "confirmed", confirmed.Status, "a frozen registration can still be confirmed")

	reopened := f.detail(t, f.act(t, f.siblings, "reopen"))
	assert.False(t, reopened.Frozen)
	rec = f.act(t, f.siblings, "reopen")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	invalid := map[string]any{"bookings": []map[string]any{{"week_id": f.offer.Weeks[0].ID, "time_model_id": f.offer.Weeks[1].TimeModels[0].ID}}}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, bookingsPath, invalid)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.Equal(t, []string{"bookings.0"}, fields, "the time model of another week is refused at its booking")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, bookingsPath, map[string]any{"bookings": []map[string]any{}})
	require.Equal(t, http.StatusBadRequest, rec.Code, "a child keeps at least one week: %s", rec.Body.String())
	foreignChild := f.registrationPath(f.again, fmt.Sprintf("/children/%d/bookings", ben.ID))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, foreignChild, rebook)
	require.Equal(t, http.StatusNotFound, rec.Code, "the child belongs to another registration: %s", rec.Body.String())

	rebooked := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, bookingsPath, rebook))
	require.Len(t, rebooked.Children[1].Bookings, 1)
	assert.Equal(t, "bis 16 Uhr", rebooked.Children[1].Bookings[0].TimeModelName)
	last := rebooked.History[len(rebooked.History)-1]
	assert.Equal(t, "rebook", last.Action)
	assert.Len(t, last.Details["before"], 2, "the history keeps both weeks Ben was booked for")
	assert.Len(t, last.Details["after"], 1)
	actions := []string{}
	for _, event := range rebooked.History {
		actions = append(actions, event.Action)
	}
	assert.Equal(t, []string{"freeze", "confirm", "reopen", "rebook"}, actions)

	// The registration deadline passes: every registration without a
	// decision of its own is frozen; the reopened one stays open.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	closed := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	closed.RegistrationOpensOn, closed.RegistrationClosesOn = "2020-01-01", "2020-12-31"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), closed)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := f.list(t, "")
	assert.False(t, registrationByID(t, list, f.siblings).Frozen, "the reopened registration stays open")
	assert.True(t, registrationByID(t, list, f.again).Frozen, "after the deadline a registration is frozen")
	rec = f.act(t, f.again, "cancel")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.False(t, f.detail(t, f.act(t, f.again, "reopen")).Frozen)
	assert.Equal(t, "cancelled", f.detail(t, f.act(t, f.again, "cancel")).Status)
}

func TestTraegerExportsHolidayRegistrationsAsCSVAndExcel(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Export Ferien")
	download := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, f.path("/export"+query), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req)
		return rec
	}

	rec := download("?format=csv")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `filename="ferienanmeldungen-winterferien-2027.csv"`)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")), "\n")
	require.Len(t, lines, 1+4, "one row per child and booked week")
	assert.True(t, strings.HasPrefix(lines[0], "Status;Eingefroren;Anmeldung Nr.;Eingegangen am;Nachname;Vorname;Geburtsdatum;Schule;Woche;"), lines[0])
	assert.NotContains(t, rec.Body.String(), "Erdnüsse", "the export carries no health information")
	assert.Contains(t, rec.Body.String(), "Feriengast (Grundschule Melle)")
	assert.Contains(t, rec.Body.String(), "Träger Export Ferien OGS Nord", "schools appear by name")
	assert.Contains(t, rec.Body.String(), fmt.Sprintf("Ja, Anmeldung Nr. %d", f.again), "the duplicate hint travels along")

	rec = download(fmt.Sprintf("?format=csv&school=%d", f.office.schools[0]))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lines = strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	assert.Len(t, lines, 1+2, "the export of one school holds only its children")
	assert.NotContains(t, rec.Body.String(), "Feriengast")

	rec = download("?format=xlsx&status=received")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(rec.Body.String(), "PK"), "an Excel workbook is a zip archive")

	rec = download("?format=pdf")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	assert.Equal(t, []string{"export", "export", "export"}, f.auditActions(t), "every export is in the audit trail")
	var formats []string
	require.NoError(t, f.db.NewRaw(`SELECT details->>'format' FROM enrollment.holiday_registration_audit
		WHERE holiday_period_id = ? ORDER BY id`, f.period.ID).Scan(context.Background(), &formats))
	assert.Equal(t, []string{"csv", "csv", "xlsx"}, formats)
}

func TestTraegerHolidayRegistrationReviewNeedsTheRightAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Grenze Ferien")
	readOnly := newTraegerOffice(t, f.db, f.traeger, "Träger Grenze Lesend", identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Grenze Fremd", identityaccess.TraegerRoleAdministration)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, f.path("")},
		{http.MethodGet, f.path("/export?format=csv")},
		{http.MethodGet, f.registrationPath(f.siblings, "")},
		{http.MethodPost, holidayConfirmPath(f.period.ID, f.siblings)},
		{http.MethodPost, f.registrationPath(f.siblings, "/freeze")},
	} {
		rec := traegerJSON(t, f.traeger, readOnly.token, request.method, request.path, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
		rec = traegerJSON(t, f.traeger, foreign.token, request.method, request.path, nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	assert.Empty(t, f.auditActions(t), "refused requests read nothing")
	assert.Equal(t, "received", f.list(t, "").Registrations[0].Status, "the foreign confirmation changed nothing")
}

func TestTraegerHolidayRegistrationListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Anmeldungen")
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, f.path(""), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	one := list()
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), siblingsRegistration(f.offer, "budget@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Len(t, list(), len(one), "the list reads the same statements for any number of registrations")
	testpkg.AssertQueryBudget(t, "api.traeger.holiday_registrations", one)
}

// TestHolidayRegistrationAuditGoesWithItsPeriod pins the retention of the
// audit trail: the hourly cleanup deletes the entries of an ended period with
// its registrations and keeps those of a later period.
func TestHolidayRegistrationAuditGoesWithItsPeriod(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Frist Protokoll")
	f.list(t, "")
	f.detail(t, f.act(t, f.siblings, "confirm"))

	body := januaryHolidays(f.office)
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2020-03-31"
	body.Weeks[0].StartsOn, body.Weeks[0].EndsOn = "2020-07-06", "2020-07-10"
	body.Weeks[1].StartsOn, body.Weeks[1].EndsOn = "2020-07-13", "2020-07-17"
	ended := createHolidayPeriod(t, f.traeger, f.office, body)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(ended.ID)+"/registrations", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		_, err := enrollmentCompose.NewHolidayRegistrationRetention().DeleteExpiredHolidayRegistrations(adminCtx)
		return err
	}))
	count := func(period int64) int {
		var rows int
		require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_audit WHERE holiday_period_id = ?`, period).
			Scan(context.Background(), &rows))
		return rows
	}
	assert.Zero(t, count(ended.ID), "the audit trail of the ended period is gone")
	assert.Equal(t, 2, count(f.period.ID), "the audit trail of the running period stays")
}
