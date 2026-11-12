// Router-level tests for the limits of the public holiday registration
// (#6726): they count stored registrations only, so families behind one
// address pass, a refused attempt takes nothing from a budget, the e-mail
// address answers with its own code, and parallel registrations cannot pass
// a limit together.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

const (
	// holidayTestAddress is the client address httptest gives every request.
	holidayTestAddress            = "192.0.2.1"
	storedRegistrationsPerIP      = 100
	storedRegistrationsPerEmail   = 10
	holidayLimitKeyIP             = "ip"
	holidayLimitKeyEmail          = "email"
	holidayRegistrationCountSQL   = `SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE organization_id = ?`
	holidayLimitBucketCountSQL    = `SELECT COALESCE(SUM(attempts), 0) FROM enrollment.holiday_registration_rate_limits WHERE organization_id = ? AND key_type = ? AND key_hash = encode(sha256(convert_to(?, 'UTF8')), 'hex')`
	holidayLimitBucketInsertSQL   = `INSERT INTO enrollment.holiday_registration_rate_limits (organization_id, key_type, key_hash, attempts, window_start) VALUES (?, ?, encode(sha256(convert_to(?, 'UTF8')), 'hex'), ?, statement_timestamp())`
	holidayEmailLimitCode         = "enrollment.submission_email_rate_limited"
	holidayAddressLimitCode       = "enrollment.submission_rate_limited"
	holidayEmailLimitRetryAfter   = "86400"
	holidayAddressLimitRetryAfter = "3600"
)

func fillHolidayBucket(t *testing.T, f holidayRegistrationFixture, organizationID int64, keyType, value string, count int) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), holidayLimitBucketInsertSQL, organizationID, keyType, value, count)
	require.NoError(t, err)
}

func holidayBucket(t *testing.T, f holidayRegistrationFixture, organizationID int64, keyType, value string) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(holidayLimitBucketCountSQL, organizationID, keyType, value).Scan(context.Background(), &count))
	return count
}

func storedHolidayRegistrations(t *testing.T, f holidayRegistrationFixture, organizationID int64) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(holidayRegistrationCountSQL, organizationID).Scan(context.Background(), &count))
	return count
}

func TestHolidayRegistrationLetsFamiliesBehindOneAddressThrough(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Schulnetz", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)

	// Twelve families of one school network: the old limit of ten attempts
	// an hour refused the eleventh.
	for i := range 12 {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, fmt.Sprintf("familie%d@example.org", i)))
		require.Equal(t, http.StatusCreated, rec.Code, "family %d: %s", i+1, rec.Body.String())
	}
	assert.Equal(t, 12, storedHolidayRegistrations(t, f, office.organization))
	assert.Equal(t, 12, holidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress))
}

func TestHolidayRegistrationRefusedAttemptsTakeNothingFromTheLimits(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Fehlversuche", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	const email = "fehlversuch@example.org"

	// More refused attempts than the e-mail address may store in a day: an
	// expired captcha and a form the parent has to correct.
	for range storedRegistrationsPerEmail {
		expired := siblingsRegistration(offer, email)
		expired["captcha_token"] = failingCaptcha
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), expired)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		incomplete := siblingsRegistration(offer, email)
		incomplete["privacy_consent"] = false
		rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), incomplete)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	assert.Zero(t, holidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress))
	assert.Zero(t, holidayBucket(t, f, office.organization, holidayLimitKeyEmail, email))

	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, email))
	require.Equal(t, http.StatusCreated, rec.Code, "the corrected registration passes: %s", rec.Body.String())
	assert.Equal(t, 1, holidayBucket(t, f, office.organization, holidayLimitKeyEmail, email))
}

func TestHolidayRegistrationEmailLimitAnswersWithItsOwnCode(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Adresssperre", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	const email = "viele@example.org"
	fillHolidayBucket(t, f, office.organization, holidayLimitKeyEmail, email, storedRegistrationsPerEmail-1)

	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, " Viele@Example.org "))
	require.Equal(t, http.StatusCreated, rec.Code, "the tenth stored registration of the day passes: %s", rec.Body.String())

	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, email))
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, holidayEmailLimitCode, code)
	assert.Equal(t, holidayEmailLimitRetryAfter, rec.Header().Get("Retry-After"), "the e-mail address is locked for a day, not an hour")

	assert.Equal(t, 1, storedHolidayRegistrations(t, f, office.organization), "the refused registration is not stored")
	assert.Equal(t, storedRegistrationsPerEmail, holidayBucket(t, f, office.organization, holidayLimitKeyEmail, email))
	assert.Equal(t, 1, holidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress),
		"the refusal rolls the count of the address back with it")

	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, "andere@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, "another family of the same address passes: %s", rec.Body.String())
}

func TestHolidayRegistrationAddressLimitRefusesWithoutAHalfRegistration(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Adressgrenze", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	fillHolidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress, storedRegistrationsPerIP)

	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, "zuviel@example.org"))
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, holidayAddressLimitCode, code)
	assert.Equal(t, holidayAddressLimitRetryAfter, rec.Header().Get("Retry-After"))

	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT
		(SELECT COUNT(*) FROM enrollment.holiday_registrations WHERE organization_id = ?)
		+ (SELECT COUNT(*) FROM enrollment.holiday_registration_children WHERE organization_id = ?)
		+ (SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ?)`,
		office.organization, office.organization, office.organization).Scan(context.Background(), &rows))
	assert.Zero(t, rows, "no registration, no child and no receipt mail of the refused registration")
	assert.Equal(t, storedRegistrationsPerIP, holidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress))
	assert.Zero(t, holidayBucket(t, f, office.organization, holidayLimitKeyEmail, "zuviel@example.org"))
}

func TestHolidayRegistrationParallelRegistrationsCannotPassTheLimitTogether(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Gleichzeitig", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	const (
		left     = 3
		families = 9
	)
	fillHolidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress, storedRegistrationsPerIP-left)

	answers := make([]*httptest.ResponseRecorder, families)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range answers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			answers[i] = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(offer, fmt.Sprintf("gleichzeitig%d@example.org", i)))
		}()
	}
	close(start)
	wg.Wait()

	stored := 0
	for i, rec := range answers {
		if rec.Code == http.StatusCreated {
			stored++
			continue
		}
		require.Equal(t, http.StatusTooManyRequests, rec.Code, "family %d: %s", i, rec.Body.String())
	}
	assert.Equal(t, left, stored, "exactly the registrations the budget had left are stored")
	assert.Equal(t, left, storedHolidayRegistrations(t, f, office.organization))
	assert.Equal(t, storedRegistrationsPerIP, holidayBucket(t, f, office.organization, holidayLimitKeyIP, holidayTestAddress))
}
