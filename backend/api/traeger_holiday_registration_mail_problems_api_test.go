package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
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

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := decodeSchoolPortalData[enrollment.HolidayRegistrationList](t, rec.Body.Bytes())
	byID := map[int64][]enrollment.HolidayRegistrationMailProblem{}
	for _, registration := range list.Registrations {
		byID[registration.ID] = registration.MailProblems
	}
	assert.Equal(t, failed, byID[f.siblings])
	assert.Equal(t, blocked, byID[f.again])

	receipt(f.siblings, `last_error = ?`, deliveryContract.DispatchOutcomeUnknown)
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailUnknown}},
		detailProblems(f.siblings))
}
