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

// #6976: the detail names every mail of the registration with its outcome,
// all five kinds, and the warning and the list name the place, closing day
// and account mails that did not arrive like the receipt. A family's closing
// day notice relates to one registration and counts for each it names. A mail
// held back at a blocked address never reads as sent. Another Träger reads
// none of it: the read of the registration already refuses it, so the mail
// read is never reached. Its own Träger filter is proved without RLS in
// TestTraegerEmailStatusesOfStaysWithTheTraeger (#6999).
func TestHolidayRegistrationDetailNamesEveryMail(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Mailstand Alle")
	ctx := context.Background()
	mail := func(kind string, related int64, key, set string, args ...any) {
		var id int64
		require.NoError(t, f.db.NewRaw(`INSERT INTO platform.email_outbox
			(organization_id, kind, recipient, payload, related_entity_type, related_entity_id, idempotency_key)
			VALUES (?, ?, '{"address":"eva.muster@example.org"}', '{}', ?, ?, ?) RETURNING id`,
			f.office.organization, kind, enrollment.MailRelatedHolidayRegistration, related, key).Scan(ctx, &id))
		_, err := f.db.NewRaw(`UPDATE platform.email_outbox SET `+set+` WHERE id = ?`, append(args, id)...).Exec(ctx)
		require.NoError(t, err)
	}
	mail(enrollment.MailKindHolidayPlaceGranted, f.siblings, "holiday-place-granted:test",
		`status = 'sent', sent_at = NOW(), delivery_state = 'delivered', delivery_state_at = NOW()`)
	mail(enrollment.MailKindHolidayParentAccountOffer, f.siblings, "holiday-parent-account-offer:test",
		`status = 'sent', sent_at = NOW()`)
	closureKey := "holiday-closure-op:test:family"
	mail(enrollment.MailKindHolidayClosureNotice, f.siblings, closureKey,
		`status = 'cancelled', last_error = ?, cancelled_at = NOW()`, deliveryContract.MailHeldByList)
	for _, registration := range []int64{f.siblings, f.again} {
		_, err := f.db.NewRaw(`INSERT INTO enrollment.holiday_closure_sends
			(organization_id, operation_id, family_key, holiday_registration_id, idempotency_key)
			VALUES (?, '00000000-0000-4000-8000-000000006976', repeat('a', 64), ?, ?)`,
			f.office.organization, registration, closureKey).Exec(ctx)
		require.NoError(t, err)
	}

	detail := func(id int64) enrollment.HolidayRegistrationDetail {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	}
	outcomes := func(mails []enrollment.RequestMailOutcome) map[string]string {
		byKind := map[string]string{}
		for _, mail := range mails {
			byKind[mail.Kind] = mail.Outcome
		}
		return byKind
	}
	siblings := detail(f.siblings)
	assert.Equal(t, map[string]string{
		enrollment.MailKindHolidayRegistrationReceived: string(deliveryContract.EmailQueued),
		enrollment.MailKindHolidayPlaceGranted:         string(deliveryContract.EmailDelivered),
		enrollment.MailKindHolidayParentAccountOffer:   string(deliveryContract.EmailAccepted),
		enrollment.MailKindHolidayClosureNotice:        string(deliveryContract.EmailBlocked),
	}, outcomes(siblings.Mails))
	for _, mail := range siblings.Mails {
		assert.Empty(t, mail.AddressMark, "a Träger keeps no address list")
	}
	closure := []enrollment.HolidayRegistrationMailProblem{{Kind: enrollment.MailKindHolidayClosureNotice, State: enrollment.HolidayMailBlocked}}
	assert.Equal(t, closure, siblings.MailProblems, "the held back notice is a problem, the sent mails are none")

	again := detail(f.again)
	assert.Equal(t, map[string]string{
		enrollment.MailKindHolidayRegistrationReceived: string(deliveryContract.EmailQueued),
		enrollment.MailKindHolidayClosureNotice:        string(deliveryContract.EmailBlocked),
	}, outcomes(again.Mails), "the family's notice names the second registration too")
	assert.Equal(t, closure, again.MailProblems)
	assert.Equal(t, map[int64][]enrollment.HolidayRegistrationMailProblem{f.siblings: closure, f.again: closure}, f.mailProblems(t))

	other := newTraegerOffice(t, f.db, f.traeger, "Mailstand Alle Fremd", identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, f.traeger, other.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "holiday_closure_notice", "another Träger reads none of the mails")
}

// #6976, the staging case of 11.11.: the mail server took the receipt, then
// the provider withheld it for a known bad address. It reads as held back,
// never as sent, in the detail, the warning and the list. A receipt the
// parents reported as unwanted and an acceptance after which they
// unsubscribed arrived: they name what the parents did, not a held mail.
func TestHolidayRegistrationMailsNameSuppressedAndRefusedMails(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Mailstand Zurückgehalten")
	ctx := context.Background()
	set := func(id int64, kind, set string) {
		_, err := f.db.NewRaw(`UPDATE platform.email_outbox SET `+set+`, delivery_state_at = NOW()
			WHERE organization_id = ? AND kind = ? AND related_entity_id = ?`, f.office.organization, kind, id).Exec(ctx)
		require.NoError(t, err)
	}
	set(f.siblings, enrollment.MailKindHolidayRegistrationReceived, `status = 'sent', sent_at = NOW(), delivery_state = 'suppressed'`)
	set(f.again, enrollment.MailKindHolidayRegistrationReceived, `status = 'sent', sent_at = NOW(), delivery_state = 'complained'`)
	_, err := f.db.NewRaw(`INSERT INTO platform.email_outbox
		(organization_id, kind, recipient, payload, related_entity_type, related_entity_id, idempotency_key, status, sent_at, delivery_state, delivery_state_at)
		VALUES (?, ?, '{"address":"papa.muster@example.org"}', '{}', ?, ?, ?, 'sent', NOW(), 'unsubscribed', NOW())`,
		f.office.organization, enrollment.MailKindHolidayRegistrationAccepted, enrollment.MailRelatedHolidayRegistration, f.again,
		"holiday-registration-accepted:test").Exec(ctx)
	require.NoError(t, err)

	detail := func(id int64) enrollment.HolidayRegistrationDetail {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	}
	siblings := detail(f.siblings)
	require.Len(t, siblings.Mails, 1)
	assert.Equal(t, string(deliveryContract.EmailProviderBlocked), siblings.Mails[0].Outcome, "a withheld mail never reads as sent")
	assert.NotNil(t, siblings.Mails[0].CreatedAt, "each mail names when it came about")
	suppressed := []enrollment.HolidayRegistrationMailProblem{{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailBlocked}}
	assert.Equal(t, suppressed, siblings.MailProblems)

	again := detail(f.again)
	refused := []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailComplained},
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, State: enrollment.HolidayMailUnsubscribed},
	}
	assert.Equal(t, refused, again.MailProblems)
	assert.Equal(t, map[int64][]enrollment.HolidayRegistrationMailProblem{f.siblings: suppressed, f.again: refused}, f.mailProblems(t),
		"the list marks both")
}
