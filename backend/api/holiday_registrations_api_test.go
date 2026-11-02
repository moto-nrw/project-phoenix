// Router-level tests for the public holiday registration (#4661, ADR 0110):
// one link per published period for all its schools, siblings and holiday
// guests, the receipt in the Träger's outbox, no child data behind a name and
// a date of birth, and the abuse protection of the online registration.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// failingCaptcha is the token the local Turnstile refuses.
const failingCaptcha = "refused-token"

type holidayRegistrationFixture struct {
	db      *bun.DB
	traeger *traegerPortalHandler
	public  http.Handler
}

// newHolidayRegistrationFixture mounts /traeger and /traeger/holiday-registrations
// as the serving root does, with a local Turnstile.
func newHolidayRegistrationFixture(t *testing.T) holidayRegistrationFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	traeger, _ := traegerPortalOn(t, db, unit)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	turnstile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		_ = json.NewEncoder(w).Encode(map[string]any{"success": r.Form.Get("response") != failingCaptcha})
	}))
	t.Cleanup(turnstile.Close)
	resource := NewHolidayRegistrationHTTP(organizations, delivery.Module, TurnstileKeys{SiteKey: "site-key", SecretKey: "secret-key"},
		turnstile.URL, testHealthCipher(), slog.Default())
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	router.Mount("/traeger/holiday-registrations", resource.Router())
	return holidayRegistrationFixture{db: db, traeger: traeger, public: router}
}

type holidayOfferBody struct {
	TraegerName    string `json:"traeger_name"`
	Name           string `json:"name"`
	Binding        bool   `json:"binding"`
	Open           bool   `json:"open"`
	CaptchaSiteKey string `json:"captcha_site_key"`
	Weeks          []struct {
		ID             int64  `json:"id"`
		StartsOn       string `json:"starts_on"`
		HostSchoolName string `json:"host_school_name"`
		TimeModels     []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			PriceCents int64  `json:"price_cents"`
		} `json:"time_models"`
	} `json:"weeks"`
	Schools []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"schools"`
}

type traegerPeriodLink struct {
	ID                int64             `json:"id"`
	PublicToken       string            `json:"public_token"`
	RegistrationCount int               `json:"registration_count"`
	Weeks             []holidayWeekBody `json:"weeks"`
}

// openHolidays is a published period whose registration window covers every
// plausible test day; its weeks lie in January 2027.
func openHolidays(t *testing.T, f holidayRegistrationFixture, office traegerOffice) traegerPeriodLink {
	t.Helper()
	body := januaryHolidays(office)
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
	created := createHolidayPeriod(t, f.traeger, office, body)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
}

func publicJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func registrationPath(token string) string { return "/traeger/holiday-registrations/" + token }

func holidayOffer(t *testing.T, f holidayRegistrationFixture, token string) holidayOfferBody {
	t.Helper()
	rec := publicJSON(t, f.public, http.MethodGet, registrationPath(token), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[holidayOfferBody](t, rec.Body.Bytes())
}

// siblingsRegistration registers a child of the Träger's first school for
// the first week and a holiday guest for both weeks.
func siblingsRegistration(offer holidayOfferBody, email string) map[string]any {
	return map[string]any{
		"guardian": map[string]any{"first_name": "Eva", "last_name": "Muster", "email": email, "phone": "0151 2345678"},
		"children": []map[string]any{
			{
				"first_name": "Lia", "last_name": "Muster", "birth_date": "2018-03-01", "home_school_id": offer.Schools[0].ID,
				"allergies": "Erdnüsse", "bookings": []map[string]any{{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[1].ID}},
			},
			{
				"first_name": "Ben", "last_name": "Muster", "birth_date": "2016-09-12", "home_school_id": 0, "guest_school_name": "Grundschule Melle",
				"bookings": []map[string]any{
					{"week_id": offer.Weeks[0].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID},
					{"week_id": offer.Weeks[1].ID, "time_model_id": offer.Weeks[1].TimeModels[0].ID},
				},
			},
		},
		"emergency_contacts": []map[string]any{{"name": "Oma Muster", "phone": "05402 1234", "relationship": "Großmutter"}},
		"pickup_persons":     []map[string]any{{"name": "Opa Muster"}},
		"privacy_consent":    true, "health_consent": true, "binding_confirmed": true,
		"captcha_token": "valid-token",
	}
}

func registrationRefusalOf(t *testing.T, rec *httptest.ResponseRecorder) (string, []string) {
	t.Helper()
	var refusal holidayErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal), rec.Body.String())
	fields := []string{}
	for _, field := range refusal.Errors {
		fields = append(fields, field.Field)
	}
	return refusal.Code, fields
}

func TestHolidayRegistrationLinkServesThePublishedPeriodForAllSchools(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Bissendorf Link", identityaccess.TraegerRoleAdministration)

	body := januaryHolidays(office)
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
	draft := createHolidayPeriod(t, f.traeger, office, body)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayPeriodPath(draft.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	link := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	require.NotEmpty(t, link.PublicToken, "the Träger-Büro sees the link of its period")

	rec = publicJSON(t, f.public, http.MethodGet, registrationPath(link.PublicToken), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "a draft has no open link: %s", rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_not_found", code)

	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(draft.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = publicJSON(t, f.public, http.MethodGet, registrationPath(link.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	offer := decodeSchoolPortalData[holidayOfferBody](t, rec.Body.Bytes())
	assert.Equal(t, "Träger Bissendorf Link", offer.TraegerName)
	assert.Equal(t, "Winterferien 2027", offer.Name)
	assert.True(t, offer.Open)
	assert.True(t, offer.Binding)
	assert.Equal(t, "site-key", offer.CaptchaSiteKey, "the captcha is always required")
	require.Len(t, offer.Weeks, 2)
	assert.Equal(t, "Träger Bissendorf Link OGS Nord", offer.Weeks[0].HostSchoolName)
	assert.Equal(t, "Träger Bissendorf Link OGS Süd", offer.Weeks[1].HostSchoolName, "one link covers the weeks of every school")
	require.Len(t, offer.Weeks[0].TimeModels, 2)
	assert.Equal(t, []int64{4500, 6750}, []int64{offer.Weeks[0].TimeModels[0].PriceCents, offer.Weeks[0].TimeModels[1].PriceCents},
		"every time model shows its price per week at the choice (#5237)")
	assert.Len(t, offer.Schools, 2)

	rec = publicJSON(t, f.public, http.MethodGet, registrationPath("unknown-token"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestHolidayRegistrationStoresSiblingsAndAHolidayGuestWithItsReceipt(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Bissendorf Anmeldung", identityaccess.TraegerRoleAdministration)
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Fremd Anmeldung", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)

	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, "eva.muster@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Erdnüsse", "the answer repeats no health information")
	var receipt struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	assert.ElementsMatch(t, []string{"id", "submitted_at"}, keysOf(receipt.Data), "the answer repeats no personal data")

	ctx := context.Background()
	var children []struct {
		FirstName       string `bun:"first_name"`
		HomeSchoolID    *int64 `bun:"home_school_id"`
		GuestSchoolName string `bun:"guest_school_name"`
		HealthData      []byte `bun:"health_data"`
		Position        int    `bun:"position"`
		Bookings        int    `bun:"bookings"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT child.first_name, child.home_school_id, child.guest_school_name, child.health_data, child.position,
			(SELECT COUNT(*) FROM enrollment.holiday_registration_bookings AS booking WHERE booking.holiday_registration_child_id = child.id) AS bookings
		FROM enrollment.holiday_registration_children AS child
		WHERE child.organization_id = ? ORDER BY child.position`, office.organization).Scan(ctx, &children))
	require.Len(t, children, 2)
	require.NotNil(t, children[0].HomeSchoolID)
	assert.Equal(t, office.schools[0], *children[0].HomeSchoolID)
	// The allergy is sealed (#4722): no plaintext in the table, and it opens
	// only with the key ring for this child's row.
	assert.NotContains(t, string(children[0].HealthData), "Erdnüsse")
	var registrationID int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registrations WHERE organization_id = ?`, office.organization).Scan(ctx, &registrationID))
	opened, err := testHealthCipher().Open(children[0].HealthData,
		enrollment.HolidayChildHealthContext(office.organization, registrationID, children[0].Position))
	require.NoError(t, err)
	assert.Contains(t, string(opened), "Erdnüsse")
	assert.Nil(t, children[1].HealthData, "a child without health information gets no sealed value")
	assert.Equal(t, 1, children[0].Bookings)
	assert.Nil(t, children[1].HomeSchoolID, "the child without a school of the Träger is a holiday guest")
	assert.Equal(t, "Grundschule Melle", children[1].GuestSchoolName)
	assert.Equal(t, 2, children[1].Bookings)
	var consents struct {
		Health  bool `bun:"health"`
		Binding bool `bun:"binding"`
		Contact int  `bun:"contacts"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT registration.health_consent_at IS NOT NULL AS health, registration.binding_confirmed_at IS NOT NULL AS binding,
			(SELECT COUNT(*) FROM enrollment.holiday_registration_contacts AS contact WHERE contact.holiday_registration_id = registration.id) AS contacts
		FROM enrollment.holiday_registrations AS registration WHERE registration.organization_id = ?`, office.organization).Scan(ctx, &consents))
	assert.True(t, consents.Health)
	assert.True(t, consents.Binding)
	assert.Equal(t, 2, consents.Contact, "one emergency contact and one person who may collect the children")

	var mail struct {
		TenantID *int64          `bun:"tenant_id"`
		Kind     string          `bun:"kind"`
		Payload  json.RawMessage `bun:"payload,type:jsonb"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT tenant_id, kind, payload FROM platform.email_outbox WHERE organization_id = ?`, office.organization).Scan(ctx, &mail))
	assert.Nil(t, mail.TenantID, "the receipt is a mail of the Träger, not of a school")
	assert.Equal(t, "holiday_registration_received", mail.Kind)
	assert.NotContains(t, string(mail.Payload), "Erdnüsse", "the receipt carries no health information")
	assert.NotContains(t, string(mail.Payload), "2018-03-01", "the receipt carries no date of birth")
	assert.Contains(t, string(mail.Payload), "Ben Muster")

	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayPeriodPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes()).RegistrationCount)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, holidayPeriodPath(period.ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger sees neither the period nor its registrations")
}

// TestHolidayRegistrationNameAndBirthDateUnlockNoChild pins user story 30: a
// registration that names a child of a school of the Träger by name and date
// of birth reads, links and returns nothing of that child.
func TestHolidayRegistrationNameAndBirthDateUnlockNoChild(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Datenschutz", identityaccess.TraegerRoleAdministration)
	student := testpkg.CreateTestStudentForTenant(t, f.db, office.schools[0], "Lia", "Muster", "2a")
	_, err := f.db.ExecContext(context.Background(), `UPDATE users.persons SET birthday = '2018-03-01' WHERE id = ?`, student.PersonID)
	require.NoError(t, err)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)

	countSchoolRows := func() int {
		var rows int
		require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT COUNT(*) FROM enrollment.requests WHERE tenant_id = ?)
			+ (SELECT COUNT(*) FROM users.guardian_profiles WHERE tenant_id = ?)`, office.schools[0], office.schools[0]).Scan(context.Background(), &rows))
		return rows
	}
	before := countSchoolRows()
	registration := siblingsRegistration(offer, "fremd@example.org")
	registration["guardian"] = map[string]any{"first_name": "Fremde", "last_name": "Person", "email": "fremd@example.org", "phone": "0170 1"}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), registration)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "2a", "no class of the student")
	assert.NotContains(t, rec.Body.String(), "Lia", "no name comes back")
	assert.Equal(t, before, countSchoolRows(), "the registration writes no enrollment request and no guardian at the school")

	offerAgain := publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, offerAgain.Code)
	assert.NotContains(t, offerAgain.Body.String(), "Lia", "the link shows no registered or known child")
}

func TestHolidayRegistrationRefusesInvalidClosedAndAbusiveSubmissions(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Prüfung Anmeldung", identityaccess.TraegerRoleAdministration)
	foreign := traegerOrganization(t, f.db, "Träger Fremdschule", true)
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, f.db, foreign, "OGS Fremdschule")
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)

	invalid := siblingsRegistration(offer, "pruefung@example.org")
	children := invalid["children"].([]map[string]any)
	children[0]["home_school_id"] = foreignSchool
	children[1]["bookings"] = []map[string]any{{"week_id": offer.Weeks[1].ID, "time_model_id": offer.Weeks[0].TimeModels[0].ID}}
	invalid["privacy_consent"] = false
	invalid["health_consent"] = false
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), invalid)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.ElementsMatch(t, []string{"children.0.home_school_id", "children.1.bookings.0", "privacy_consent", "health_consent"}, fields)

	tooMany := siblingsRegistration(offer, "pruefung@example.org")
	sibling := tooMany["children"].([]map[string]any)[1]
	tooMany["children"] = []map[string]any{sibling, sibling, sibling, sibling}
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), tooMany)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields = registrationRefusalOf(t, rec)
	assert.Equal(t, []string{"children"}, fields, "at most three siblings")

	refused := siblingsRegistration(offer, "pruefung@example.org")
	refused["captcha_token"] = failingCaptcha
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), refused)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.captcha_failed", code)

	// Three attempts counted so far; the online registration's limit for an
	// e-mail address is five a day, so the sixth is refused.
	for range 2 {
		rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), refused)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, "pruefung@example.org"))
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.submission_rate_limited", code)
	assert.Equal(t, "3600", rec.Header().Get("Retry-After"))
	var stored int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE organization_id = ?`, office.organization).Scan(context.Background(), &stored))
	assert.Zero(t, stored, "no refused attempt stored a registration")

	closedBody := januaryHolidays(office)
	closedBody.RegistrationOpensOn, closedBody.RegistrationClosesOn = "2020-01-01", "2020-01-31"
	closed := createHolidayPeriod(t, f.traeger, office, closedBody)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(closed.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	closedLink := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	closedOffer := holidayOffer(t, f, closedLink.PublicToken)
	assert.False(t, closedOffer.Open, "the link tells that the registration window is over")
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(closedLink.PublicToken), siblingsRegistration(closedOffer, "spaet@example.org"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_closed", code)
}

func TestHolidayWeekWithRegistrationsCannotBeRemoved(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Umplanung", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	registration := siblingsRegistration(offer, "umplanung@example.org")
	registration["children"] = registration["children"].([]map[string]any)[:1]
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), registration)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, holidayPeriodPath(period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	current := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())

	withoutBookedWeek := current
	withoutBookedWeek.Weeks = []holidayWeekBody{current.Weeks[1]}
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(period.ID), withoutBookedWeek)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_week_booked", code)

	withoutBookedModel := current
	withoutBookedModel.Weeks = []holidayWeekBody{current.Weeks[0], current.Weeks[1]}
	withoutBookedModel.Weeks[0].TimeModels = current.Weeks[0].TimeModels[:1]
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(period.ID), withoutBookedModel)
	require.Equal(t, http.StatusConflict, rec.Code, "the booked time model stays: %s", rec.Body.String())

	withoutFreeWeek := current
	withoutFreeWeek.Weeks = []holidayWeekBody{current.Weeks[0]}
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayPeriodPath(period.ID), withoutFreeWeek)
	require.Equal(t, http.StatusOK, rec.Code, "a week without registrations may go: %s", rec.Body.String())
}

func keysOf(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

// TestHolidayRegistrationsGoThreeMonthsAfterTheirPeriod pins user story 121:
// the hourly retention deletes the registrations of a period whose last week
// ended more than 92 days ago, with children, bookings and contacts, and
// keeps those of a later period.
func TestHolidayRegistrationsGoThreeMonthsAfterTheirPeriod(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Löschfrist", identityaccess.TraegerRoleAdministration)
	register := func(firstWeek, secondWeek [2]string) int64 {
		body := januaryHolidays(office)
		body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
		body.Weeks[0].StartsOn, body.Weeks[0].EndsOn = firstWeek[0], firstWeek[1]
		body.Weeks[1].StartsOn, body.Weeks[1].EndsOn = secondWeek[0], secondWeek[1]
		created := createHolidayPeriod(t, f.traeger, office, body)
		rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		period := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
		offer := holidayOffer(t, f, period.PublicToken)
		rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, "frist@example.org"))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		return period.ID
	}
	ended := register([2]string{"2020-07-06", "2020-07-10"}, [2]string{"2020-07-13", "2020-07-17"})
	later := register([2]string{"2099-07-06", "2099-07-10"}, [2]string{"2099-07-13", "2099-07-17"})

	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		_, err := enrollmentCompose.NewHolidayRegistrationRetention(nil).DeleteExpiredHolidayRegistrations(adminCtx)
		return err
	}))
	countRows := func(period int64) int {
		var rows int
		require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE holiday_period_id = ?)
			+ (SELECT COUNT(*) FROM enrollment.holiday_registration_children WHERE holiday_period_id = ?)
			+ (SELECT COUNT(*) FROM enrollment.holiday_registration_bookings WHERE holiday_period_id = ?)`, period, period, period).Scan(context.Background(), &rows))
		return rows
	}
	assert.Zero(t, countRows(ended), "the registrations of the ended period are gone with their children and bookings")
	assert.Equal(t, 1+2+3, countRows(later), "the registration of the later period stays")
}
