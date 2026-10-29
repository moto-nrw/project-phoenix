package api

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHolidayRegistrationSecondGuardianValidation(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Zweite Person Prüfung", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	for index, tc := range []struct {
		name, field string
		guardian    map[string]any
	}{
		{"only first name", "last_name", map[string]any{"first_name": "Tom"}},
		{"only last name", "first_name", map[string]any{"last_name": "Muster"}},
		{"invalid email", "email", map[string]any{"first_name": "Tom", "last_name": "Muster", "email": "invalid"}},
		{"invalid phone", "phone", map[string]any{"first_name": "Tom", "last_name": "Muster", "phone": "abc"}},
		{"name too long", "first_name", map[string]any{"first_name": strings.Repeat("ä", 101), "last_name": "Muster"}},
		{"phone too long", "phone", map[string]any{"first_name": "Tom", "last_name": "Muster", "phone": strings.Repeat("1", 41)}},
		{"email too long", "email", map[string]any{"first_name": "Tom", "last_name": "Muster", "email": strings.Repeat("a", 245) + "@example.org"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := siblingsRegistration(offer, fmt.Sprintf("validation%d@example.org", index))
			body["second_guardian"] = tc.guardian
			rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			code, fields := registrationRefusalOf(t, rec)
			assert.Equal(t, "enrollment.holiday_registration_invalid", code)
			assert.Contains(t, fields, "second_guardian."+tc.field)
		})
	}
}

func TestHolidayRegistrationSecondGuardianStaysOptional(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Zweite Person freiwillig", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	for i, second := range []any{nil, map[string]any{"first_name": " ", "email": " "}, map[string]any{"first_name": "Tom", "last_name": "Muster"}} {
		body := siblingsRegistration(offer, fmt.Sprintf("optional%d@example.org", i))
		if second != nil {
			body["second_guardian"] = second
		}
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		id := decodeSchoolPortalData[struct {
			ID int64 `json:"id"`
		}](t, rec.Body.Bytes()).ID
		rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, fmt.Sprintf("%s/registrations/%d", holidayPeriodPath(period.ID), id), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		detail := decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
		if i < 2 {
			assert.Nil(t, detail.SecondGuardian)
		} else {
			require.NotNil(t, detail.SecondGuardian)
			assert.Equal(t, enrollment.HolidayGuardianInput{FirstName: "Tom", LastName: "Muster"}, *detail.SecondGuardian)
		}
	}
}

func TestHolidayRegistrationSecondGuardianIsAuditedAndOnlyInDetailAndExport(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Zweite Person Zugriff")
	body := siblingsRegistration(f.offer, "new@example.org")
	body["second_guardian"] = map[string]any{"first_name": " Tom ", "last_name": " Anders ", "email": " TOM@EXAMPLE.ORG ", "phone": " +49 1512345678 "}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
	assert.NotContains(t, rec.Body.String(), "Anders")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	require.NotNil(t, detail.SecondGuardian)
	assert.Equal(t, enrollment.HolidayGuardianInput{FirstName: "Tom", LastName: "Anders", Email: "tom@example.org", Phone: "+49 1512345678"}, *detail.SecondGuardian)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "second_guardian")
	assert.NotContains(t, rec.Body.String(), "Anders")
	var mail string
	require.NoError(t, f.db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND related_entity_id = ?
  AND kind = 'holiday_registration_received'`, f.office.organization, id).Scan(context.Background(), &mail))
	assert.Contains(t, mail, "Eva Muster")
	assert.NotContains(t, mail, "Anders")
	assertSecondGuardianExports(t, f)
	assert.Contains(t, f.auditActions(t), fmt.Sprintf("read:%d", id))
	assert.Contains(t, f.auditActions(t), "list")
	assert.Contains(t, f.auditActions(t), "export")
	for _, role := range []string{identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRoleAdministration} {
		other := newTraegerOffice(t, f.db, f.traeger, "Zweite Person Fremdzugriff "+role, role)
		rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, f.registrationPath(id, ""), nil)
		if role == identityaccess.TraegerRoleReadOnly {
			assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		} else {
			assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		}
	}
}

func assertSecondGuardianExports(t *testing.T, f reviewFixture) {
	t.Helper()
	for _, format := range []string{"csv", "xlsx"} {
		req := httptest.NewRequest(http.MethodGet, f.path("/export?format="+format), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		if format == "csv" {
			assert.Contains(t, rec.Body.String(), "Zweites Elternteil;Telefon zweites Elternteil")
			assert.Contains(t, rec.Body.String(), "Tom Anders;'+49 1512345678")
		} else {
			archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
			require.NoError(t, err)
			var content bytes.Buffer
			for _, file := range archive.File {
				reader, err := file.Open()
				require.NoError(t, err)
				_, err = io.Copy(&content, reader)
				require.NoError(t, err)
				require.NoError(t, reader.Close())
			}
			assert.Contains(t, content.String(), "Zweites Elternteil")
			assert.Contains(t, content.String(), "Telefon zweites Elternteil")
			assert.Contains(t, content.String(), "Tom Anders")
			assert.Contains(t, content.String(), "+49 1512345678")
		}
	}
}

func TestHolidaySecondGuardianStaysOutOfSchoolParticipantsAndGuestCopy(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Zweite Person Schulgrenze")
	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET
  second_guardian_first_name = 'Tom', second_guardian_last_name = 'Anders',
  second_guardian_email = 'tom@example.org', second_guardian_phone = '0151 9999999' WHERE id = ?`, f.siblings)
	require.NoError(t, err)
	f.confirm(t, f.siblings, "confirm")
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)
	rec := f.get(t, lead, fmt.Sprintf("/%d/participants", f.offer.Weeks[0].ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Ben")
	assert.NotContains(t, rec.Body.String(), "Anders")
	assert.NotContains(t, rec.Body.String(), "second_guardian")
	rec = f.assign(t, lead, f.offer.Weeks[0].ID, f.group)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	guest := f.roster(t, calendar.NewDate(2099, 7, 8))["Ben Muster"]
	require.NotZero(t, guest)
	rec = f.sheet(t, lead, guest)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Eva Muster")
	for _, private := range []string{"Anders", "tom@example.org", "0151 9999999", "second_guardian"} {
		assert.NotContains(t, rec.Body.String(), private)
	}
}
