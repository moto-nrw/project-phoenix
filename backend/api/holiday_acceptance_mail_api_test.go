// Router-level tests for the acceptance mail of a holiday registration
// (#6672): the Träger writes the text of the mail at the period, a
// confirmation puts exactly one mail per registration on the outbox in the
// same write, and a cancellation after the confirmation sends no second mail.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type acceptanceMailRow struct {
	TenantID  *int64          `bun:"tenant_id"`
	RelatedID int64           `bun:"related_entity_id"`
	Payload   json.RawMessage `bun:"payload,type:jsonb"`
}

type acceptanceMailPayload struct {
	Recipient   string `json:"recipient_email"`
	TraegerName string `json:"traeger_name"`
	PeriodName  string `json:"period_name"`
	TraegerText string `json:"traeger_text"`
	Children    []struct {
		Name  string   `json:"name"`
		Guest bool     `json:"guest"`
		Weeks []string `json:"weeks"`
	} `json:"children"`
}

func acceptanceMails(t *testing.T, f reviewFixture, registrationID int64) []acceptanceMailRow {
	t.Helper()
	var rows []acceptanceMailRow
	require.NoError(t, f.db.NewRaw(`SELECT tenant_id, related_entity_id, payload FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'holiday_registration_accepted' AND related_entity_type = 'holiday_registration'
			AND related_entity_id = ?`, f.office.organization, registrationID).Scan(context.Background(), &rows))
	return rows
}

func (f reviewFixture) writeAcceptanceText(t *testing.T, text string) (int, holidayPeriodBody) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayPeriodPath(f.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	body.AcceptanceMailText = text
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayPeriodPath(f.period.ID), body)
	if rec.Code != http.StatusOK {
		return rec.Code, holidayPeriodBody{}
	}
	return rec.Code, decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
}

func TestConfirmationSendsOneAcceptanceMailWithTheTextOfTheTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Annahme Mail")

	code, _ := f.writeAcceptanceText(t, strings.Repeat("ä", 2001))
	assert.Equal(t, http.StatusBadRequest, code, "the text of the mail is bounded")

	text := "Bitte Mittagessen mitbringen.\nWechselkleidung <b>nicht</b> vergessen."
	code, saved := f.writeAcceptanceText(t, "  "+strings.ReplaceAll(text, "\n", "\r\n")+"\n")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, text, saved.AcceptanceMailText, "the period keeps the text trimmed, with its lines")

	unique := testpkg.UniqueTestTenantID(t)
	parent := fmt.Sprintf("annahme.%d@example.org", unique)
	submit := func(email string) int64 {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), siblingsRegistration(f.offer, email))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[struct {
			ID int64 `json:"id"`
		}](t, rec.Body.Bytes()).ID
	}
	plain, other := submit(parent), submit(fmt.Sprintf("zweite.%d@example.org", unique))

	assert.Empty(t, acceptanceMails(t, f, plain), "a received registration has no acceptance mail")
	require.Equal(t, http.StatusOK, f.act(t, plain, "confirm").Code)
	mails := acceptanceMails(t, f, plain)
	require.Len(t, mails, 1, "the confirmation puts exactly one acceptance mail on the outbox")
	assert.Nil(t, mails[0].TenantID, "the acceptance is a mail of the Träger, not of a school")
	var payload acceptanceMailPayload
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, parent, payload.Recipient, "the mail goes to the parent who registered")
	assert.Equal(t, text, payload.TraegerText)
	assert.Equal(t, "Winterferien 2027", payload.PeriodName)
	assert.NotContains(t, string(mails[0].Payload), "parent_app_url", "the link to the parents portal is decided at sending")
	require.Len(t, payload.Children, 2)
	assert.Equal(t, "Lia Muster", payload.Children[0].Name)
	require.Len(t, payload.Children[0].Weeks, 1)
	assert.Contains(t, payload.Children[0].Weeks[0], "bis 16 Uhr (08:00 bis 16:00 Uhr)", "the week names its time model")
	assert.True(t, payload.Children[1].Guest)
	assert.Len(t, payload.Children[1].Weeks, 2)
	assert.NotContains(t, string(mails[0].Payload), "Erdnüsse", "the mail carries no health information")
	assert.NotContains(t, string(mails[0].Payload), "2018-03-01", "the mail carries no date of birth")

	assert.Empty(t, acceptanceMails(t, f, other), "only the confirmed registration gets its mail")

	assert.NotEqual(t, http.StatusOK, f.act(t, plain, "confirm").Code, "a confirmed registration is not confirmed again")
	require.Equal(t, http.StatusOK, f.act(t, plain, "cancel").Code)
	assert.Len(t, acceptanceMails(t, f, plain), 1, "neither a second confirmation nor a cancellation sends another acceptance mail")
}

func TestConfirmationWithoutTextSendsTheFixedPart(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Annahme ohne Text")
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	mails := acceptanceMails(t, f, f.siblings)
	require.Len(t, mails, 1, "a period without a text still sends the mail")
	var payload acceptanceMailPayload
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Empty(t, payload.TraegerText)
	assert.NotEmpty(t, payload.Children)
}
