package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

// #6737: the Träger-Büro sees in the list and the detail of a registration
// when its receipt mail did not reach the parents: given up, held back for a
// blocked address, or with an unknown outcome. A mail on its way shows nothing.
func TestHolidayRegistrationReviewNamesUndeliveredMails(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Mailstand Prüfung")
	detailProblems := func(id int64) []enrollment.HolidayRegistrationMailProblem {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes()).MailProblems
	}
	assert.Empty(t, detailProblems(f.siblings), "a waiting receipt is no problem")

	receipt := func(id int64, set string, args ...any) {
		_, err := f.db.NewRaw(`UPDATE platform.email_outbox SET `+set+`
			WHERE organization_id = ? AND kind = 'holiday_registration_received' AND related_entity_id = ?`,
			append(args, f.office.organization, id)...).Exec(context.Background())
		require.NoError(t, err)
	}
	receipt(f.siblings, `status = 'dead_letter', attempts = 6, last_error = 'provider rejected', dead_letter_at = NOW()`)
	receipt(f.again, `status = 'cancelled', last_error = ?, cancelled_at = NOW()`, deliveryContract.MailHeldByList)

	failed := []enrollment.HolidayRegistrationMailProblem{{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailFailed}}
	blocked := []enrollment.HolidayRegistrationMailProblem{{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailBlocked}}
	assert.Equal(t, failed, detailProblems(f.siblings))
	assert.Equal(t, blocked, detailProblems(f.again))

	assert.Equal(t, map[int64][]enrollment.HolidayRegistrationMailProblem{f.siblings: failed, f.again: blocked}, f.mailProblems(t),
		"the list reads them for the whole period in one answer")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "mail_problems", "the list itself keeps its statements")

	receipt(f.siblings, `last_error = ?`, deliveryContract.DispatchOutcomeUnknown)
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailUnknown}},
		detailProblems(f.siblings))
}

// mailProblems reads the undelivered mails of the period as the list does.
func (f reviewFixture) mailProblems(t *testing.T) map[int64][]enrollment.HolidayRegistrationMailProblem {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path("/mail-problems"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	answer := decodeSchoolPortalData[struct {
		Registrations []enrollment.HolidayRegistrationMails `json:"registrations"`
	}](t, rec.Body.Bytes())
	problems := map[int64][]enrollment.HolidayRegistrationMailProblem{}
	for _, registration := range answer.Registrations {
		problems[registration.RegistrationID] = registration.MailProblems
	}
	return problems
}

// The mails of another Träger's period stay out of reach, and the read is
// flat in the number of registrations (#6737).
func TestHolidayRegistrationMailProblemsStayWithTheTraegerAndAreFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Mailstand Budget")
	other := newTraegerOffice(t, f.db, f.traeger, "Mailstand Fremd", identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, f.traeger, other.token, http.MethodGet, f.path("/mail-problems"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	readOnly := newTraegerOffice(t, f.db, f.traeger, "Mailstand Lesend", identityaccess.TraegerRoleReadOnly)
	rec = traegerJSON(t, f.traeger, readOnly.token, http.MethodGet, f.path("/mail-problems"), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the read needs the right to manage registrations")

	read := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, f.path("/mail-problems"), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := read()
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), siblingsRegistration(f.offer, "mailbudget@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Len(t, read().Statements, len(one.Statements), "the read is flat in the number of registrations")
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_registration_mail_problems", one)
}
