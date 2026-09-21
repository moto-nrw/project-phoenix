package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rules below mirror the model validators the retained repositories ran
// before every write, so the stored rows stay identical.

func TestNormalizeGuardianLinkMatchesTheRetainedValidation(t *testing.T) {
	t.Parallel()
	base := GuardianLinkRecord{StudentID: 7, GuardianProfileID: 9}

	link := base
	link.RelationshipType = " Parent "
	got, err := NormalizeGuardianLink(link)
	require.NoError(t, err)
	assert.Equal(t, " parent ", got.RelationshipType, "lower-cased but not trimmed")
	assert.Equal(t, DefaultGuardianRole, got.GuardianRole)
	assert.JSONEq(t, `{}`, string(got.Permissions))
	assert.Equal(t, 1, got.EmergencyPriority, "an unset priority takes the column default")

	link.EmergencyPriority = -2
	got, err = NormalizeGuardianLink(link)
	require.NoError(t, err)
	assert.Equal(t, -2, got.EmergencyPriority, "a set priority is stored as given")

	link = base
	_, err = NormalizeGuardianLink(link)
	require.EqualError(t, err, "relationship type is required")
	link.RelationshipType = "  "
	_, err = NormalizeGuardianLink(link)
	require.EqualError(t, err, "invalid relationship type")
	link.RelationshipType = "parent"
	link.Permissions = json.RawMessage(`[]`)
	_, err = NormalizeGuardianLink(link)
	require.ErrorIs(t, err, ErrGuardianInvalid)
}

func TestNormalizeGuardianContactAndPhoneMatchTheRetainedValidation(t *testing.T) {
	t.Parallel()
	blank, empty := "  ", ""
	_, err := NormalizeGuardianContact(GuardianContact{Email: &blank})
	require.EqualError(t, err, "invalid email format", "a blank e-mail is refused, as before")
	got, err := NormalizeGuardianContact(GuardianContact{Email: &empty, PreferredContactMethod: "sms"})
	require.NoError(t, err)
	assert.Equal(t, "", *got.Email, "an empty e-mail is stored as given")

	phone, err := NormalizeGuardianPhone(GuardianPhoneRecord{PhoneNumber: " 030 12 ", PhoneType: "home", Priority: -3, Label: &blank})
	require.NoError(t, err)
	assert.Equal(t, "030 12", phone.PhoneNumber)
	assert.Equal(t, 1, phone.Priority)
	assert.Nil(t, phone.Label)
	_, err = NormalizeGuardianPhone(GuardianPhoneRecord{PhoneNumber: "030 12", PhoneType: ""})
	require.ErrorIs(t, err, ErrGuardianInvalid, "an empty type is refused, as before")
}
